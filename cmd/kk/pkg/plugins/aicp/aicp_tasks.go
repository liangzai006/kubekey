package aicp

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	kkkubernetes "github.com/kubesphere/kubekey/v3/cmd/kk/pkg/client/kubernetes"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/registry"

	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type AicpStorageTask struct {
	common.KubeAction
}

func (a *AicpStorageTask) Execute(runtime connector.Runtime) error {
	aicpStorageDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "charts", "aicp-storage")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "aicp-storage",
		Namespace: "aicp-storage",
		ChartPath: aicpStorageDir,
		Values:    vals,
	}
	return helm.Install()
}

type GenerateAicpAkSkTask struct {
	common.KubeAction
}

func (i *GenerateAicpAkSkTask) Execute(runtime connector.Runtime) error {
	// if cluster file contains iaas keys, use it
	iaasKeys := i.KubeConf.Cluster.Aicp.IaasKeys
	if len(iaasKeys) > 0 {
		i.PipelineCache.Set(common.IAAS_AKSK, iaasKeys)
		return nil
	}
	// if not, use k8s client to get it
	k8sClient, err := kkkubernetes.NewClient(filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	iaasAkskConfig, err := k8sClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "iaas-ak-sk", metav1.GetOptions{})
	if err == nil {
		i.PipelineCache.Set(common.IAAS_AKSK, iaasAkskConfig.Data)
		return nil
	}

	// if not, use docker client to get it
	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		return err
	}
	pullImage := fmt.Sprintf("%s/aicp/ubuntu:gen-key", i.KubeConf.Cluster.Registry.PrivateRegistry)

	pull, err := dockerClient.ImagePull(ctx, pullImage, dockerTypes.ImagePullOptions{})
	if err != nil {
		return err
	}

	io.Copy(io.Discard, pull)
	defer pull.Close()

	iaasAksk := make(map[string]string)

	for _, s := range []string{"CONSOLE", "BOSS", "ADMIN"} {
		container, err := dockerClient.ContainerCreate(ctx, &container.Config{
			Image: pullImage,
		}, nil, nil, nil, "generate-ak-sk")
		if err != nil {
			return err
		}

		err = dockerClient.ContainerStart(ctx, container.ID, dockerTypes.ContainerStartOptions{})
		if err != nil {
			return err
		}

		logs, err := dockerClient.ContainerLogs(ctx, container.ID, dockerTypes.ContainerLogsOptions{
			ShowStdout: true,
			Follow:     true,
		})
		if err != nil {
			return err
		}

		logBytes, err := io.ReadAll(logs)
		if err != nil {
			return err
		}

		// filter bad characters
		cleanLog := make([]rune, 0, len(logBytes))
		for _, b := range logBytes {
			if b >= 32 && b <= 126 {
				if unicode.IsPrint(rune(b)) {
					cleanLog = append(cleanLog, rune(b))
				}
			}
		}
		klog.Infof("%s key: %s", s, string(cleanLog))

		// parse key and secret key
		fields := strings.Fields(string(cleanLog))
		iaasAksk[fmt.Sprintf("%s_KEY_ID", s)] = fields[0]
		iaasAksk[fmt.Sprintf("%s_SECRET_KEY", s)] = fields[1]
		iaasAksk[fmt.Sprintf("%s_SECRET_CONSOLE_KEY", s)] = fields[2]

		logs.Close()
		dockerClient.ContainerRemove(ctx, container.ID, dockerTypes.ContainerRemoveOptions{
			Force: true,
		})
	}

	// set iaas keys to config map
	_, err = k8sClient.CoreV1().ConfigMaps("kube-system").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "iaas-ak-sk",
		},
		Data: iaasAksk,
	}, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	i.PipelineCache.Set(common.IAAS_AKSK, iaasAksk)

	return nil
}

type CertManagerTask struct {
	common.KubeAction
}

func (c *CertManagerTask) Execute(runtime connector.Runtime) error {
	certManagerDir := filepath.Join(c.KubeConf.Arg.AicpWorkDir, "charts", "cert-manager")
	certManager := &HelmOptions{
		Name:      "cert-manager",
		Namespace: "cert-manager",
		ChartPath: certManagerDir,
		Values: map[string]interface{}{
			"global": map[string]interface{}{
				"offlineRepo": c.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
	}
	return certManager.Install()
}

type IstioTask struct {
	common.KubeAction
}

func (i *IstioTask) Execute(runtime connector.Runtime) error {
	istioDir := filepath.Join(i.KubeConf.Arg.AicpWorkDir, "charts", "istio")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": i.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}

	helm := HelmOptions{
		Name:      "istiod",
		Namespace: "istio-system",
		ChartPath: istioDir,
		Values:    vals,
		PreHook: func(ctx context.Context, kubeClient kubernetes.Interface) error {

			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-system",
					Labels: map[string]string{
						"istio-injection":        "disabled",
						"istio-operator-managed": "Reconcile",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierror.IsAlreadyExists(err) {
				return err
			}

			return nil
		},
	}
	return helm.Install()
}

type ClusterLocalGatewayTask struct {
	common.KubeAction
}

func (c *ClusterLocalGatewayTask) Execute(runtime connector.Runtime) error {

	clusterLocalGatewayDir := filepath.Join(c.KubeConf.Arg.AicpWorkDir, "charts", "cluster-local-gateway")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": c.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "cluster-local-gateway",
		Namespace: "istio-system",
		ChartPath: clusterLocalGatewayDir,
		Values:    vals,
	}
	return helm.Install()
}

type KubeflowTask struct {
	common.KubeAction
}

func (k *KubeflowTask) Execute(runtime connector.Runtime) error {
	kubeflowDir := filepath.Join(k.KubeConf.Arg.AicpWorkDir, "charts", "kubeflow")

	helm := HelmOptions{
		Name:      "kubeflow",
		Namespace: "kubeflow",
		ChartPath: kubeflowDir,
		PreHook: func(ctx context.Context, kubeClient kubernetes.Interface) error {
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeflow",
					Labels: map[string]string{
						"istio-injection": "enabled",
						"control-plane":   "kubeflow",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierror.IsAlreadyExists(err) {
				return err
			}

			return nil
		},
	}
	return helm.Install()

}

type AuthServerTask struct {
	common.KubeAction
}

func (a *AuthServerTask) Execute(runtime connector.Runtime) error {
	authServerDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "charts", "auth-server")
	iaasKeys, ok := a.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain": a.KubeConf.Cluster.Aicp.Domain,
			"iaas": map[string]interface{}{
				"zone":            a.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
		},
	}
	helm := HelmOptions{
		Name:      "auth-server",
		Namespace: "istio-system",
		ChartPath: authServerDir,
		Values:    vals,
	}
	return helm.Install()
}

type PodDefaultsTask struct {
	common.KubeAction
}

func (p *PodDefaultsTask) Execute(runtime connector.Runtime) error {
	podDefaultsDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "poddefaults")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "poddefaults",
		Namespace: "kubeflow",
		ChartPath: podDefaultsDir,
		Values:    vals,
	}
	return helm.Install()
}

type NotebookControllerTask struct {
	common.KubeAction
}

func (n *NotebookControllerTask) Execute(runtime connector.Runtime) error {
	notebookControllerDir := filepath.Join(n.KubeConf.Arg.AicpWorkDir, "charts", "notebook-controller")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"zone": n.KubeConf.Cluster.Aicp.Zone,
		},
	}

	helm := HelmOptions{
		Name:      "notebook-controller",
		Namespace: "kubeflow",
		ChartPath: notebookControllerDir,
		Values:    vals,
	}
	return helm.Install()
}

type ProfilesTask struct {
	common.KubeAction
}

func (p *ProfilesTask) Execute(runtime connector.Runtime) error {
	profilesDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "profiles")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}

	helm := HelmOptions{
		Name:      "profiles",
		Namespace: "kubeflow",
		ChartPath: profilesDir,
		Values:    vals,
	}
	return helm.Install()
}

type TensorboardControllerTask struct {
	common.KubeAction
}

func (t *TensorboardControllerTask) Execute(runtime connector.Runtime) error {
	tensorboardControllerDir := filepath.Join(t.KubeConf.Arg.AicpWorkDir, "charts", "tensorboard-controller")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": t.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"zone": t.KubeConf.Cluster.Aicp.Zone,
		},
	}
	helm := HelmOptions{
		Name:      "tensorboard-controller",
		Namespace: "kubeflow",
		ChartPath: tensorboardControllerDir,
		Values:    vals,
	}
	return helm.Install()
}

type TrainingOperatorTask struct {
	common.KubeAction
}

func (t *TrainingOperatorTask) Execute(runtime connector.Runtime) error {
	trainingOperatorDir := filepath.Join(t.KubeConf.Arg.AicpWorkDir, "charts", "training-operator")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": t.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "training-operator",
		Namespace: "kubeflow",
		ChartPath: trainingOperatorDir,
		Values:    vals,
	}
	return helm.Install()
}

type AicpWebAppTask struct {
	common.KubeAction
}

func (a *AicpWebAppTask) Execute(runtime connector.Runtime) error {
	aicpWebAppDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "charts", "aicp-web-app")

	iaasKeys, ok := a.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	auths := registry.DockerRegistryAuthEntries(a.KubeConf.Cluster.Registry.Auths)
	if _, ok := auths[a.KubeConf.Cluster.Registry.GetHost()]; !ok {
		return fmt.Errorf("registry auth not found: %s", a.KubeConf.Cluster.Registry.GetHost())
	}
	auth := auths[a.KubeConf.Cluster.Registry.GetHost()]

	protocol := "https"
	if auth.PlainHTTP {
		protocol = "http"
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain":  a.KubeConf.Cluster.Aicp.Domain,
			"sshHost": a.KubeConf.Cluster.ControlPlaneEndpoint.Address,
			"billing": strconv.FormatBool(a.KubeConf.Cluster.Aicp.Billing),
			"iaas": map[string]interface{}{
				"zone":            a.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
			"docker": map[string]interface{}{
				"protocol": protocol,
				"username": auth.Username,
				"password": auth.Password,
			},
		},
	}
	helm := HelmOptions{
		Name:      "aicp-web-app",
		Namespace: "aicp-system",
		ChartPath: aicpWebAppDir,
		Values:    vals,
		PreHook: func(ctx context.Context, kubeClient kubernetes.Interface) error {
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "aicp-system",
					Labels: map[string]string{
						"istio-injection": "enabled",
						"control-plane":   "aicp-system",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierror.IsAlreadyExists(err) {
				return err
			}

			return nil
		},
	}
	return helm.Install()
}

type ImagebuilderTask struct {
	common.KubeAction
}

func (i *ImagebuilderTask) Execute(runtime connector.Runtime) error {
	imagebuilderDir := filepath.Join(i.KubeConf.Arg.AicpWorkDir, "charts", "imagebuilder")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": i.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "imagebuilder",
		Namespace: "aicp-system",
		ChartPath: imagebuilderDir,
		Values:    vals,
	}
	return helm.Install()
}

type EpfsTask struct {
	common.KubeAction
}

func (e *EpfsTask) Execute(runtime connector.Runtime) error {
	epfsDir := filepath.Join(e.KubeConf.Arg.AicpWorkDir, "charts", "epfs")
	iaasKeys, ok := e.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": e.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain":  e.KubeConf.Cluster.Aicp.Domain,
			"billing": FormatBilling(e.KubeConf.Cluster.Aicp.Billing),
			"iaas": map[string]interface{}{
				"zone":            e.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
		},
	}
	helm := HelmOptions{
		Name:      "epfs",
		Namespace: "aicp-system",
		ChartPath: epfsDir,
		Values:    vals,
	}
	return helm.Install()
}

type PushServerTask struct {
	common.KubeAction
}

func (p *PushServerTask) Execute(runtime connector.Runtime) error {
	pushServerDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "push-server")
	iaasKeys, ok := p.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain": p.KubeConf.Cluster.Aicp.Domain,
			"iaas": map[string]interface{}{
				"zone":            p.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
		},
	}
	helm := HelmOptions{
		Name:      "push-server",
		Namespace: "aicp-system",
		ChartPath: pushServerDir,
		Values:    vals,
	}
	return helm.Install()
}

type DockerApiServerTask struct {
	common.KubeAction
}

func (d *DockerApiServerTask) Execute(runtime connector.Runtime) error {
	dockerApiServerDir := filepath.Join(d.KubeConf.Arg.AicpWorkDir, "charts", "docker-api-server")

	iaasKeys, ok := d.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	auths := registry.DockerRegistryAuthEntries(d.KubeConf.Cluster.Registry.Auths)
	if _, ok := auths[d.KubeConf.Cluster.Registry.GetHost()]; !ok {
		return fmt.Errorf("registry auth not found: %s", d.KubeConf.Cluster.Registry.GetHost())
	}
	auth := auths[d.KubeConf.Cluster.Registry.GetHost()]
	protocol := "https"
	if auth.PlainHTTP {
		protocol = "http"
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain": d.KubeConf.Cluster.Aicp.Domain,
			"iaas": map[string]interface{}{
				"zone":            d.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
			"docker": map[string]interface{}{
				"protocol": protocol,
				"host":     d.KubeConf.Cluster.Registry.GetHost(),
				"username": auth.Username,
				"password": auth.Password,
			},
		},
	}
	helm := HelmOptions{
		Name:      "docker-api-server",
		Namespace: "aicp-system",
		ChartPath: dockerApiServerDir,
		Values:    vals,
	}
	return helm.Install()
}

type ResourceProxyTask struct {
	common.KubeAction
}

func (r *ResourceProxyTask) Execute(runtime connector.Runtime) error {
	resourceProxyDir := filepath.Join(r.KubeConf.Arg.AicpWorkDir, "charts", "resource-proxy")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": r.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "resource-proxy",
		Namespace: "aicp-resource",
		ChartPath: resourceProxyDir,
		Values:    vals,
	}
	return helm.Install()
}

type PrometheusBlackboxExporterTask struct {
	common.KubeAction
}

func (p *PrometheusBlackboxExporterTask) Execute(runtime connector.Runtime) error {
	prometheusBlackboxExporterDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "prometheus-blackbox-exporter")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"imageRegistry": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"serviceMonitor": map[string]interface{}{
			"enabled":             true,
			"selfMonitor.enabled": true,
		},
	}
	helm := HelmOptions{
		Name:      "prometheus-blackbox-exporter",
		Namespace: "kubesphere-monitoring-system",
		ChartPath: prometheusBlackboxExporterDir,
		Values:    vals,
	}
	return helm.Install()
}

type MaasTask struct {
	common.KubeAction
}

func (m *MaasTask) Execute(runtime connector.Runtime) error {
	maasDir := filepath.Join(m.KubeConf.Arg.AicpWorkDir, "charts", "maas")
	iaasKeys, ok := m.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	auths := registry.DockerRegistryAuthEntries(m.KubeConf.Cluster.Registry.Auths)
	if _, ok := auths[m.KubeConf.Cluster.Registry.GetHost()]; !ok {
		return fmt.Errorf("registry auth not found: %s", m.KubeConf.Cluster.Registry.GetHost())
	}
	auth := auths[m.KubeConf.Cluster.Registry.GetHost()]
	protocol := "https"
	if auth.PlainHTTP {
		protocol = "http"
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": m.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain":  m.KubeConf.Cluster.Aicp.Domain,
			"billing": FormatBilling(m.KubeConf.Cluster.Aicp.Billing),
			"iaas": map[string]interface{}{
				"zone":            m.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
			"docker": map[string]interface{}{
				"protocol": protocol,
				"host":     m.KubeConf.Cluster.Registry.GetHost(),
				"username": auth.Username,
				"password": auth.Password,
			},
		},
	}
	helm := HelmOptions{
		Name:      "maas",
		Namespace: "maas-system",
		ChartPath: maasDir,
		Values:    vals,
		PreHook: func(ctx context.Context, kubeClient kubernetes.Interface) error {
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "maas-system",
					Labels: map[string]string{
						"istio-injection": "enabled",
						"control-plane":   "maas-system",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierror.IsAlreadyExists(err) {
				return err
			}

			return nil
		},
	}
	return helm.Install()
}

type OperationTask struct {
	common.KubeAction
}

func (o *OperationTask) Execute(runtime connector.Runtime) error {
	operationDir := filepath.Join(o.KubeConf.Arg.AicpWorkDir, "charts", "operation-record")
	iaasKeys, ok := o.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"imageRegistry": o.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain": o.KubeConf.Cluster.Aicp.Domain,
			"iaas": map[string]interface{}{
				"zone":            o.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
		},
	}
	helm := HelmOptions{
		Name:      "operation",
		Namespace: "aicp-system",
		ChartPath: operationDir,
		Values:    vals,
	}
	return helm.Install()
}

type LwsTask struct {
	common.KubeAction
}

func (l *LwsTask) Execute(runtime connector.Runtime) error {
	lwsDir := filepath.Join(l.KubeConf.Arg.AicpWorkDir, "charts", "lws")

	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"registry": l.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "lws",
		Namespace: "lws-system",
		ChartPath: lwsDir,
		Values:    vals,
	}
	return helm.Install()
}

type VolcanoTask struct {
	common.KubeAction
}

func (v *VolcanoTask) Execute(runtime connector.Runtime) error {
	volcanoDir := filepath.Join(v.KubeConf.Arg.AicpWorkDir, "charts", "volcano")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"repository": v.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "volcano",
		Namespace: "volcano-system",
		ChartPath: volcanoDir,
		Values:    vals,
	}
	return helm.Install()
}
