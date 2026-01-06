/*
 Copyright 2021 The KubeSphere Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package kubesphere

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/mohae/deepcopy"

	"github.com/pkg/errors"
	yamlV3 "gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog/v2"

	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	ksTemp "github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubesphere/templates"
	ksv2 "github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubesphere/v2"
	ksv3 "github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubesphere/v3"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/aicp"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/version/kubesphere"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/version/kubesphere/templates"
)

type AddInstallerConfig struct {
	common.KubeAction
}

func (a *AddInstallerConfig) Execute(runtime connector.Runtime) error {
	configurationBase64 := base64.StdEncoding.EncodeToString([]byte(a.KubeConf.Cluster.KubeSphere.Configurations))
	if _, err := runtime.GetRunner().SudoCmd(
		fmt.Sprintf("echo %s | base64 -d >> /etc/kubernetes/addons/kubesphere.yaml", configurationBase64),
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), "add config to ks-installer manifests failed")
	}
	return nil
}

type CreateNamespace struct {
	common.KubeAction
}

func (c *CreateNamespace) Execute(runtime connector.Runtime) error {
	_, err := runtime.GetRunner().SudoCmd(`cat <<EOF | /usr/local/bin/kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: kubesphere-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: kubesphere-monitoring-system
EOF
`, false)
	if err != nil {
		return errors.Wrap(errors.WithStack(err), "create namespace: kubesphere-system and kubesphere-monitoring-system")
	}
	return nil
}

type Setup struct {
	common.KubeAction
}

func (s *Setup) Execute(runtime connector.Runtime) error {
	filePath := filepath.Join(common.KubeAddonsDir, templates.KsInstaller.Name())

	var addrList []string
	var tlsDisable bool
	var port string
	switch s.KubeConf.Cluster.Etcd.Type {
	case kubekeyapiv1alpha2.KubeKey:
		for _, host := range runtime.GetHostsByRole(common.ETCD) {
			addrList = append(addrList, host.GetInternalIPv4Address())
		}

		caFile := "/etc/ssl/etcd/ssl/ca.pem"
		certFile := fmt.Sprintf("/etc/ssl/etcd/ssl/node-%s.pem", runtime.RemoteHost().GetName())
		keyFile := fmt.Sprintf("/etc/ssl/etcd/ssl/node-%s-key.pem", runtime.RemoteHost().GetName())
		if output, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("/usr/local/bin/kubectl -n kubesphere-monitoring-system create secret generic kube-etcd-client-certs "+
				"--from-file=etcd-client-ca.crt=%s "+
				"--from-file=etcd-client.crt=%s "+
				"--from-file=etcd-client.key=%s", caFile, certFile, keyFile), true); err != nil {
			if !strings.Contains(output, "exists") {
				return err
			}
		}
	case kubekeyapiv1alpha2.Kubeadm:
		for _, host := range runtime.GetHostsByRole(common.Master) {
			addrList = append(addrList, host.GetInternalIPv4Address())
		}

		caFile := "/etc/kubernetes/pki/etcd/ca.crt"
		certFile := "/etc/kubernetes/pki/etcd/healthcheck-client.crt"
		keyFile := "/etc/kubernetes/pki/etcd/healthcheck-client.key"
		if output, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("/usr/local/bin/kubectl -n kubesphere-monitoring-system create secret generic kube-etcd-client-certs "+
				"--from-file=etcd-client-ca.crt=%s "+
				"--from-file=etcd-client.crt=%s "+
				"--from-file=etcd-client.key=%s", caFile, certFile, keyFile), true); err != nil {
			if !strings.Contains(output, "exists") {
				return err
			}
		}
	case kubekeyapiv1alpha2.External:
		for _, endpoint := range s.KubeConf.Cluster.Etcd.External.Endpoints {
			e := strings.Split(strings.TrimSpace(endpoint), "://")
			s := strings.Split(e[1], ":")
			port = s[1]
			addrList = append(addrList, s[0])
			if e[0] == "http" {
				tlsDisable = true
			}
		}
		if tlsDisable {
			if output, err := runtime.GetRunner().SudoCmd("/usr/local/bin/kubectl -n kubesphere-monitoring-system create secret generic kube-etcd-client-certs", true); err != nil {
				if !strings.Contains(output, "exists") {
					return err
				}
			}
		} else {
			caFile := fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(s.KubeConf.Cluster.Etcd.External.CAFile))
			certFile := fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(s.KubeConf.Cluster.Etcd.External.CertFile))
			keyFile := fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(s.KubeConf.Cluster.Etcd.External.KeyFile))
			if output, err := runtime.GetRunner().SudoCmd(
				fmt.Sprintf("/usr/local/bin/kubectl -n kubesphere-monitoring-system create secret generic kube-etcd-client-certs "+
					"--from-file=etcd-client-ca.crt=%s "+
					"--from-file=etcd-client.crt=%s "+
					"--from-file=etcd-client.key=%s", caFile, certFile, keyFile), true); err != nil {
				if !strings.Contains(output, "exists") {
					return err
				}
			}
		}
	}

	etcdEndPoint := strings.Join(addrList, ",")
	if _, err := runtime.GetRunner().SudoCmd(
		fmt.Sprintf("sed -i '/endpointIps/s/\\:.*/\\: %s/g' %s", etcdEndPoint, filePath),
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("update etcd endpoint failed"))
	}

	if tlsDisable {
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/tlsEnable/s/\\:.*/\\: false/g' %s", filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("update etcd tls failed"))
		}
	}

	if len(port) != 0 {
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i 's/2379/%s/g' %s", port, filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("update etcd tls failed"))
		}
	}

	if s.KubeConf.Cluster.Registry.PrivateRegistry != "" {
		PrivateRegistry := strings.Replace(s.KubeConf.Cluster.Registry.PrivateRegistry, "/", "\\/", -1)
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/local_registry/s/\\:.*/\\: %s/g' %s", PrivateRegistry, filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("add private registry: %s failed", s.KubeConf.Cluster.Registry.PrivateRegistry))
		}
	} else {
		if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("sed -i '/local_registry/d' %s", filePath), false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("remove private registry failed"))
		}
	}

	if s.KubeConf.Cluster.Registry.NamespaceOverride != "" {
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/namespace_override/s/\\:.*/\\: %s/g' %s", s.KubeConf.Cluster.Registry.NamespaceOverride, filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("add namespace override: %s failed", s.KubeConf.Cluster.Registry.NamespaceOverride))
		}
	} else {
		if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("sed -i '/namespace_override/d' %s", filePath), false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("remove namespace override failed"))
		}
	}

	_, ok := kubesphere.CNSource[s.KubeConf.Cluster.KubeSphere.Version]
	if ok && (os.Getenv("KKZONE") == "cn" || s.KubeConf.Cluster.Registry.PrivateRegistry == "registry.cn-beijing.aliyuncs.com") {
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/zone/s/\\:.*/\\: %s/g' %s", "cn", filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("add kubekey zone: %s failed", s.KubeConf.Cluster.Registry.PrivateRegistry))
		}
	} else {
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/zone/d' %s", filePath),
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("remove kubekey zone failed"))
		}
	}

	switch s.KubeConf.Cluster.Kubernetes.ContainerManager {
	case "docker", "containerd", "crio":
		if _, err := runtime.GetRunner().SudoCmd(
			fmt.Sprintf("sed -i '/containerruntime/s/\\:.*/\\: %s/g' /etc/kubernetes/addons/kubesphere.yaml", s.KubeConf.Cluster.Kubernetes.ContainerManager), false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("set container runtime: %s failed", s.KubeConf.Cluster.Kubernetes.ContainerManager))
		}
	default:
		logger.Log.Message(runtime.RemoteHost().GetName(),
			fmt.Sprintf("Currently, the logging module of KubeSphere does not support %s. If %s is used, the logging module will be unavailable.",
				s.KubeConf.Cluster.Kubernetes.ContainerManager, s.KubeConf.Cluster.Kubernetes.ContainerManager))
	}

	return nil
}

type Apply struct {
	common.KubeAction
}

func (a *Apply) Execute(runtime connector.Runtime) error {
	filePath := filepath.Join(common.KubeAddonsDir, templates.KsInstaller.Name())

	deployKubesphereCmd := fmt.Sprintf("/usr/local/bin/kubectl apply -f %s --force", filePath)
	if _, err := runtime.GetRunner().SudoCmd(deployKubesphereCmd, true); err != nil {
		return errors.Wrapf(errors.WithStack(err), "deploy %s failed", filePath)
	}
	return nil
}

type Check struct {
	common.KubeAction
}

func (c *Check) Execute(runtime connector.Runtime) error {
	var (
		position = 1
		notes    = "Please wait for the installation to complete: "
	)

	ch := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go CheckKubeSphereStatus(ctx, runtime, ch)

	stop := false
	for !stop {
		select {
		case res := <-ch:
			fmt.Printf("\033[%dA\033[K", position)
			fmt.Println(res)
			stop = true
		default:
			for i := 0; i < 10; i++ {
				if i < 5 {
					fmt.Printf("\033[%dA\033[K", position)

					output := fmt.Sprintf(
						"%s%s%s",
						notes,
						strings.Repeat(" ", i),
						">>--->",
					)

					fmt.Printf("%s \033[K\n", output)
					time.Sleep(time.Duration(200) * time.Millisecond)
				} else {
					fmt.Printf("\033[%dA\033[K", position)

					output := fmt.Sprintf(
						"%s%s%s",
						notes,
						strings.Repeat(" ", 10-i),
						"<---<<",
					)

					fmt.Printf("%s \033[K\n", output)
					time.Sleep(time.Duration(200) * time.Millisecond)
				}
			}
		}
	}
	return nil
}

func CheckKubeSphereStatus(ctx context.Context, runtime connector.Runtime, stopChan chan string) {
	defer close(stopChan)
	for {
		select {
		case <-ctx.Done():
			stopChan <- ""
		default:
			_, err := runtime.GetRunner().SudoCmd(
				"/usr/local/bin/kubectl exec -n kubesphere-system "+
					"$(kubectl get pod -n kubesphere-system -l app=ks-installer -o jsonpath='{.items[0].metadata.name}') "+
					"-- ls /kubesphere/playbooks/kubesphere_running", false)
			if err == nil {
				output, err := runtime.GetRunner().SudoCmd(
					"/usr/local/bin/kubectl exec -n kubesphere-system "+
						"$(kubectl get pod -n kubesphere-system -l app=ks-installer -o jsonpath='{.items[0].metadata.name}') "+
						"-- cat /kubesphere/playbooks/kubesphere_running", false)
				if err == nil && output != "" {
					stopChan <- output
					break
				}
			}
		}
	}
}

type CleanCC struct {
	common.KubeAction
}

func (c *CleanCC) Execute(runtime connector.Runtime) error {
	c.KubeConf.Cluster.KubeSphere.Configurations = "\n"
	return nil
}

type ConvertV2ToV3 struct {
	common.KubeAction
}

func (c *ConvertV2ToV3) Execute(runtime connector.Runtime) error {
	configV2Str, err := runtime.GetRunner().SudoCmd(
		"/usr/local/bin/kubectl get cm -n kubesphere-system ks-installer -o jsonpath='{.data.ks-config\\.yaml}'",
		false)
	if err != nil {
		return err
	}

	clusterCfgV2 := ksv2.V2{}
	clusterCfgV3 := ksv3.V3{}
	if err := yamlV3.Unmarshal([]byte(configV2Str), &clusterCfgV2); err != nil {
		return err
	}

	configV3, err := MigrateConfig2to3(&clusterCfgV2, &clusterCfgV3)
	if err != nil {
		return err
	}
	c.KubeConf.Cluster.KubeSphere.Configurations = "---\n" + configV3
	return nil
}

func MigrateConfig2to3(v2 *ksv2.V2, v3 *ksv3.V3) (string, error) {
	v3.Etcd = ksv3.Etcd(v2.Etcd)
	v3.Persistence = ksv3.Persistence(v2.Persistence)
	v3.Alerting = ksv3.Alerting(v2.Alerting)
	v3.Notification = ksv3.Notification(v2.Notification)
	v3.LocalRegistry = v2.LocalRegistry
	v3.Servicemesh = ksv3.Servicemesh(v2.Servicemesh)
	v3.Devops = ksv3.Devops(v2.Devops)
	v3.Openpitrix = ksv3.Openpitrix(v2.Openpitrix)
	v3.Console = ksv3.Console(v2.Console)

	if v2.MetricsServerNew.Enabled == "" {
		if v2.MetricsServerOld.Enabled == "true" || v2.MetricsServerOld.Enabled == "True" {
			v3.MetricsServer.Enabled = true
		} else {
			v3.MetricsServer.Enabled = false
		}
	} else {
		if v2.MetricsServerNew.Enabled == "true" || v2.MetricsServerNew.Enabled == "True" {
			v3.MetricsServer.Enabled = true
		} else {
			v3.MetricsServer.Enabled = false
		}
	}

	v3.Monitoring.PrometheusMemoryRequest = v2.Monitoring.PrometheusMemoryRequest
	//v3.Monitoring.PrometheusReplicas = v2.Monitoring.PrometheusReplicas
	v3.Monitoring.PrometheusVolumeSize = v2.Monitoring.PrometheusVolumeSize
	//v3.Monitoring.AlertmanagerReplicas = 1

	v3.Common.EtcdVolumeSize = v2.Common.EtcdVolumeSize
	v3.Common.MinioVolumeSize = v2.Common.MinioVolumeSize
	v3.Common.MysqlVolumeSize = v2.Common.MysqlVolumeSize
	v3.Common.OpenldapVolumeSize = v2.Common.OpenldapVolumeSize
	v3.Common.RedisVolumSize = v2.Common.RedisVolumSize
	//v3.Common.ES.ElasticsearchDataReplicas = v2.Logging.ElasticsearchDataReplicas
	//v3.Common.ES.ElasticsearchMasterReplicas = v2.Logging.ElasticsearchMasterReplicas
	v3.Common.ES.ElkPrefix = v2.Logging.ElkPrefix
	v3.Common.ES.LogMaxAge = v2.Logging.LogMaxAge
	if v2.Logging.ElasticsearchVolumeSize == "" {
		v3.Common.ES.ElasticsearchDataVolumeSize = v2.Logging.ElasticsearchDataVolumeSize
		v3.Common.ES.ElasticsearchMasterVolumeSize = v2.Logging.ElasticsearchMasterVolumeSize
	} else {
		v3.Common.ES.ElasticsearchMasterVolumeSize = "4Gi"
		v3.Common.ES.ElasticsearchDataVolumeSize = v2.Logging.ElasticsearchVolumeSize
	}

	v3.Logging.Enabled = v2.Logging.Enabled
	v3.Logging.LogsidecarReplicas = v2.Logging.LogsidecarReplicas

	v3.Authentication.JwtSecret = ""
	v3.Multicluster.ClusterRole = "none"
	v3.Events.Ruler.Replicas = 2

	var clusterConfiguration = ksv3.ClusterConfig{
		ApiVersion: "installer.kubesphere.io/v1alpha1",
		Kind:       "ClusterConfiguration",
		Metadata: ksv3.Metadata{
			Name:      "ks-installer",
			Namespace: "kubesphere-system",
			Label:     ksv3.Label{Version: "v3.0.0"},
		},
		Spec: v3,
	}

	configV3, err := yamlV3.Marshal(clusterConfiguration)
	if err != nil {
		return "", err
	}

	return string(configV3), nil
}

type DeployKsCore struct {
	common.KubeAction
}

func (d *DeployKsCore) Execute(runtime connector.Runtime) error {
	ksCoreDir := filepath.Join(d.KubeConf.Arg.AicpWorkDir, "common", "ks-core")
	keys, ok := d.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"tag":           "v4.1.2",
			"imageRegistry": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"extension": map[string]interface{}{
			"imageRegistry": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"ha": map[string]interface{}{
			"enabled": true,
			"cache": map[string]interface{}{
				"options": map[string]interface{}{
					"password": keys.(map[string]string)[common.REDIS_PASSWORD],
				},
			},
		},
		"aicp": map[string]interface{}{
			"protocol":   d.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
			"docsPrefix": d.KubeConf.Cluster.Aicp.DomainConfig.Docs,
			"domain":     d.KubeConf.Cluster.Aicp.DomainConfig.Domain,
		},
	}

	helm := aicp.HelmOptions{
		Name:      "ks-core",
		Namespace: "kubesphere-system",
		ChartPath: ksCoreDir,
		Values:    vals,
	}
	return helm.Install()

}

type PushKseExtensionTask struct {
	common.KubeAction
}

func (p *PushKseExtensionTask) Execute(runtime connector.Runtime) error {
	extensionDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "common", "kse-extensions-publish")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"imageRegistry": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"museum": map[string]interface{}{
			"enabled": true,
		},
	}

	helm := aicp.HelmOptions{
		Name:      "kse-extensions-publish",
		Namespace: "kubesphere-system",
		ChartPath: extensionDir,
		Values:    vals,
	}

	rel, err := helm.Templates()
	if err != nil {
		return err
	}

	cli := cli.New()
	kc := kube.New(cli.RESTClientGetter())
	resources, err := kc.Build(strings.NewReader(rel.Manifest), false)
	if err != nil {
		return err
	}

	for _, r := range resources {
		helper := resource.NewHelper(r.Client, r.Mapping)

		_, err = helper.Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}
		_, err = helper.Create(r.Namespace, true, r.Object)
		if err != nil {
			return err
		}
	}

	return nil

}

type PushAicpExtensionTask struct {
	common.KubeAction
}

func (p *PushAicpExtensionTask) Execute(runtime connector.Runtime) error {
	ksbuildDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "common", "kube-aicp-extensions")

	var ext *kubekeyapiv1alpha2.ExtensionKsbuilder
	ext, err := Load(ksbuildDir)
	if err != nil {
		return err
	}

	cli := cli.New()
	kc := kube.New(cli.RESTClientGetter())

	for _, obj := range ext.ToKubernetesResources() {

		data, err := yaml.Marshal(obj)
		if err != nil {
			return err
		}

		resources, err := kc.Build(bytes.NewReader(data), false)
		if err != nil {
			return err
		}
		r := resources[0]
		_, err = resource.NewHelper(r.Client, r.Mapping).Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}

		fmt.Printf("creating %s %s\n", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		_, err = kc.Create(resources)
		if err != nil {
			return err
		}
	}

	return nil
}

type ApplyInstallPlanTask struct {
	common.KubeAction
}

// 定义优先级顺序
var resourcePriority = map[string]int{
	// "kubesphere-logging-system":  1,
	"vector":                     1,
	"opensearch-internal-secret": 2,
	"opensearch":                 3,
	"whizard-telemetry":          4,
	"whizard-monitoring":         5,
	"whizard-alerting":           6,
	"whizard-notification":       7,
}
var waitResource = map[string]int{
	"opensearch":           1,
	"vector":               2,
	"whizard-telemetry":    3,
	"whizard-monitoring":   4,
	"whizard-alerting":     5,
	"whizard-notification": 6,
}

func (p *ApplyInstallPlanTask) Execute(runtime connector.Runtime) error {
	installPlanDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "common", "kse-extensions")

	fs, err := os.ReadDir(installPlanDir)
	if err != nil {
		return err
	}
	cli := cli.New()
	kc := kube.New(cli.RESTClientGetter())
	var resources kube.ResourceList

	for _, f := range fs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		yamlData, err := os.ReadFile(filepath.Join(installPlanDir, f.Name()))
		if err != nil {
			return err
		}

		kcRes, err := kc.Build(bytes.NewReader(yamlData), false)
		if err != nil {
			return err
		}
		resources = append(resources, kcRes...)

	}

	// Sort resources, giving priority to opensearch, vector, and whizard-telemetry
	sort.Slice(resources, func(i, j int) bool {
		nameI := resources[i].Name
		nameJ := resources[j].Name

		// Get priority, if not exist, set to 999 (lowest priority)
		priorityI, existsI := resourcePriority[nameI]
		priorityJ, existsJ := resourcePriority[nameJ]

		if !existsI {
			priorityI = 999
		}
		if !existsJ {
			priorityJ = 999
		}

		// If priority is different, sort by priority
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}

		// If priority is the same, sort by name alphabetically
		return nameI < nameJ
	})

	for _, r := range resources {

		helper := resource.NewHelper(r.Client, r.Mapping).WithFieldManager("coreshub-deploy")
		_, err = helper.Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}

		_, err = helper.Create(r.Namespace, true, r.Object)
		if err != nil {
			return err
		}

		_, waitResourceIsExist := waitResource[r.Name]

		if waitResourceIsExist {
			err = WaitForResource(func() (bool, error) {
				resource, err := helper.Get(r.Namespace, r.Name)
				if err != nil {
					return false, err
				}
				state, err := getStatusState(resource.(*unstructured.Unstructured))
				if err != nil {
					return false, err
				}
				return state, nil
			}, 300*time.Second)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

type ApplyPrometheusResourceTask struct {
	common.KubeAction
}

func (p *ApplyPrometheusResourceTask) Execute(runtime connector.Runtime) error {
	installPlanDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "common", "kse-extension-notification-webhook")

	fs, err := os.ReadDir(installPlanDir)
	if err != nil {
		return err
	}
	cli := cli.New()
	kc := kube.New(cli.RESTClientGetter())
	var resources kube.ResourceList

	for _, f := range fs {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}
		yamlData, err := os.ReadFile(filepath.Join(installPlanDir, f.Name()))
		if err != nil {
			return err
		}

		kcRes, err := kc.Build(bytes.NewReader(yamlData), false)
		if err != nil {
			return err
		}
		resources = append(resources, kcRes...)

	}

	for _, r := range resources {

		helper := resource.NewHelper(r.Client, r.Mapping).WithFieldManager("prometheus-resources")
		_, err = helper.Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}

		_, err = helper.Create(r.Namespace, true, r.Object)
		if err != nil {
			return err
		}

	}

	return nil
}

type ApplyMemberClusterTask struct {
	common.KubeAction
}

func (a *ApplyMemberClusterTask) Execute(runtime connector.Runtime) error {
	template := ksTemp.Cluster
	configTemplate := ksTemp.Config

	configTemplateStr, err := util.Render(configTemplate, map[string]interface{}{
		"ImageRegistry": a.KubeConf.Cluster.Registry.PrivateRegistry,
	})
	if err != nil {
		return err
	}

	configTemplateStrBase64 := base64.StdEncoding.EncodeToString([]byte(configTemplateStr))
	data := util.Data{
		"ClusterName": a.KubeConf.ClusterName,
		"Config":      configTemplateStrBase64,
		"KubeConfig":  ksTemp.GetKubeConfig(a.KubeConf),
	}
	rendered, err := util.Render(template, data)
	if err != nil {
		return err
	}

	hostKubeConfig := a.KubeConf.Arg.KubeConfig

	cli := cli.New()
	cli.KubeConfig = hostKubeConfig
	kb := kube.New(cli.RESTClientGetter())

	resources, err := kb.Build(strings.NewReader(rendered), false)
	if err != nil {
		return err
	}
	for _, r := range resources {
		help := resource.NewHelper(r.Client, r.Mapping).WithFieldManager("apply-member-cluster")
		_, err = help.Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}

		_, err = help.Create(r.Namespace, false, r.Object)
		if err != nil {
			return err
		}

		// 等待 Cluster 就绪
		err = WaitForResource(func() (bool, error) {
			cluster, err := help.Get(r.Namespace, r.Name)
			if err != nil {
				return false, err
			}

			// 检查 status.conditions
			status, ok := cluster.(*unstructured.Unstructured).Object["status"].(map[string]interface{})
			if !ok {
				klog.Infof("Cluster %s: status not found yet", r.Name)
				return false, nil
			}

			conditions, ok := status["conditions"].([]interface{})
			if !ok {
				klog.Infof("Cluster %s: conditions not found yet", r.Name)
				return false, nil
			}

			// 检查 KSCoreReady 和 Ready 两个 condition
			ksCoreReady := false
			clusterReady := false

			for _, cond := range conditions {
				condition := cond.(map[string]interface{})
				condType, _ := condition["type"].(string)
				condStatus, _ := condition["status"].(string)

				if condType == "KSCoreReady" && condStatus == "True" {
					ksCoreReady = true
				}
				if condType == "Ready" && condStatus == "True" {
					clusterReady = true
				}
			}

			klog.Infof("Cluster %s status: KSCoreReady=%v, Ready=%v", r.Name, ksCoreReady, clusterReady)

			if ksCoreReady && clusterReady {
				klog.Infof("Cluster %s is ready!", r.Name)
				return true, nil
			}

			return false, nil
		}, 500*time.Second)

		if err != nil {
			return fmt.Errorf("failed to wait for Cluster %s to be ready: %w", r.Name, err)
		}

	}
	return nil
}

type PrePatchInstallPlanTask struct {
	common.KubeAction
}

func (p *PrePatchInstallPlanTask) Execute(runtime connector.Runtime) error {
	opensearchSecretPath := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "common", "kse-extensions", "opensearch-internal-secret.yaml")

	f, err := os.OpenFile(opensearchSecretPath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	currentClusterCli := cli.New()
	currentClusterKube := kube.New(currentClusterCli.RESTClientGetter())
	resources, err := currentClusterKube.Build(f, false)
	if err != nil {
		return err
	}

	for _, r := range resources {
		helper := resource.NewHelper(r.Client, r.Mapping).WithFieldManager("apply-opensearch-secret")
		_, err = helper.Get(r.Namespace, r.Name)
		if err == nil {
			continue
		}

		_, err = helper.Create(r.Namespace, false, r.Object)
		if err != nil {
			return err
		}
	}
	return nil
}

type PatchInstallPlanTask struct {
	common.KubeAction
}

func (p *PatchInstallPlanTask) Execute(runtime connector.Runtime) error {

	cli := cli.New()
	cli.KubeConfig = p.KubeConf.Arg.KubeConfig
	kb := kube.New(cli.RESTClientGetter())
	dynamicClient, err := kb.Factory.DynamicClient()
	if err != nil {
		return err
	}

	installPlanResource := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "kubesphere.io",
		Version:  "v1alpha1",
		Resource: "installplans",
	})

	memberClusterName := p.KubeConf.ClusterName

	installPlanNames := []string{
		"opensearch",
		"vector",
		"whizard-monitoring",
		"whizard-alerting",
		"whizard-logging",
		"whizard-telemetry-ruler",
		"whizard-events",
		"whizard-auditing",
		"aicp",
	}

	for _, planName := range installPlanNames {

		plan, err := installPlanResource.Get(context.Background(), planName, metav1.GetOptions{})
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				klog.Warningf("InstallPlan %s not found, skipping", planName)
				continue
			}
			return err
		}

		clusters, _, err := unstructured.NestedStringSlice(plan.Object, "spec", "clusterScheduling", "placement", "clusters")
		if err != nil {
			return err
		}
		memberExists := false
		for _, cluster := range clusters {
			if cluster == memberClusterName {
				memberExists = true
				break
			}
		}

		if !memberExists {
			patchData := []map[string]interface{}{
				{
					"op":    "add",
					"path":  "/spec/clusterScheduling/placement/clusters/-",
					"value": memberClusterName,
				},
			}

			patchBytes, err := json.Marshal(patchData)
			if err != nil {
				return err
			}

			_, err = installPlanResource.Patch(
				context.Background(),
				planName,
				types.JSONPatchType,
				patchBytes,
				metav1.PatchOptions{},
			)
			if err != nil {
				return err
			}
			klog.Infof("Added member cluster %s to InstallPlan %s", memberClusterName, planName)

			err = WaitForResource(func() (bool, error) {
				plan, err := installPlanResource.Get(context.Background(), planName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				state, err := getStatusState(plan)
				if err != nil {
					return false, err
				}

				return state, nil
			}, 500*time.Second)
			if err != nil {
				return err
			}

		} else {
			klog.Infof("Member cluster %s already exists in InstallPlan %s", memberClusterName, planName)
		}
	}

	telemetryPlan, err := installPlanResource.Get(context.Background(), "whizard-telemetry", metav1.GetOptions{})
	if err != nil {
		return err
	}

	var configStr string
	if spec, ok := telemetryPlan.Object["spec"].(map[string]interface{}); ok {
		if config, ok := spec["config"].(string); ok {
			configStr = config
		}
	}

	if configStr == "" {
		return fmt.Errorf("InstallPlan whizard-telemetry has no config")
	}

	configMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(configStr), &configMap)
	if err != nil {
		return fmt.Errorf("failed to parse config YAML: %w", err)
	}

	newEndpoint := fmt.Sprintf("https://%s:30920", p.KubeConf.Cluster.ControlPlaneEndpoint.Address)
	if telemetryConfig, ok := configMap["whizard-telemetry"].(map[string]interface{}); ok {
		if config, ok := telemetryConfig["config"].(map[string]interface{}); ok {

			if auditingConfig, ok := config["auditing"].(map[string]interface{}); ok {
				if servers, ok := auditingConfig["servers"].([]interface{}); ok && len(servers) > 0 {

					originalServer := servers[0].(map[string]interface{})

					newServer := deepcopy.Copy(originalServer).(map[string]interface{})

					if elasticsearch, ok := newServer["elasticsearch"].(map[string]interface{}); ok {
						elasticsearch["endpoints"] = []interface{}{newEndpoint}
						elasticsearch["cluster"] = []string{memberClusterName}
					}

					auditingConfig["servers"] = append(servers, newServer)
					klog.Infof("Added new server to auditing.servers")
				}
			}

			if eventsConfig, ok := config["events"].(map[string]interface{}); ok {
				if servers, ok := eventsConfig["servers"].([]interface{}); ok && len(servers) > 0 {

					originalServer := servers[0].(map[string]interface{})

					newServer := deepcopy.Copy(originalServer).(map[string]interface{})

					if elasticsearch, ok := newServer["elasticsearch"].(map[string]interface{}); ok {
						elasticsearch["endpoints"] = []interface{}{newEndpoint}
						elasticsearch["cluster"] = []string{memberClusterName}
					}
					eventsConfig["servers"] = append(servers, newServer)
					klog.Infof("Added new server to events.servers")
				}
			}

			if loggingConfig, ok := config["logging"].(map[string]interface{}); ok {
				if servers, ok := loggingConfig["servers"].([]interface{}); ok && len(servers) > 0 {

					originalServer := servers[0].(map[string]interface{})

					newServer := deepcopy.Copy(originalServer).(map[string]interface{})

					if elasticsearch, ok := newServer["elasticsearch"].(map[string]interface{}); ok {
						elasticsearch["endpoints"] = []interface{}{newEndpoint}
						elasticsearch["cluster"] = []string{memberClusterName}
					}
					loggingConfig["servers"] = append(servers, newServer)
					klog.Infof("Added new server to logging.servers")
				}
			}
		}
	}

	newConfigBytes, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal config map: %w", err)
	}
	newConfigStr := string(newConfigBytes)

	patchData := []map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/spec/config",
			"value": newConfigStr,
		},
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %w", err)
	}

	_, err = installPlanResource.Patch(
		context.Background(),
		"whizard-telemetry",
		types.JSONPatchType,
		patchBytes,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to patch InstallPlan: %w", err)
	}

	klog.Infof("Successfully updated whizard-telemetry InstallPlan with new servers and endpoints")

	return nil
}
