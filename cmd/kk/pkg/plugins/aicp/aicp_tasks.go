package aicp

import (
	"context"
	"fmt"
	"net"
	"strings"

	"path/filepath"
	"strconv"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/registry"

	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ConfigServerTask struct {
	common.KubeAction
	member bool
}

func (c *ConfigServerTask) Execute(runtime connector.Runtime) error {
	configServerDir := filepath.Join(c.KubeConf.Arg.AicpWorkDir, "charts", "config-server")
	iaasKeys, ok := c.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	auths := registry.DockerRegistryAuthEntries(c.KubeConf.Cluster.Registry.Auths)
	if _, ok := auths[c.KubeConf.Cluster.Registry.GetHost()]; !ok {
		return fmt.Errorf("registry auth not found: %s", c.KubeConf.Cluster.Registry.GetHost())
	}
	auth := auths[c.KubeConf.Cluster.Registry.GetHost()]

	protocol := "https"
	if auth.PlainHTTP {
		protocol = "http"
	}
	clusterName := "host"
	if c.member {
		clusterName = c.KubeConf.ClusterName
	}
	ks := map[string]interface{}{
		"host": "ks-console.kubesphere-system.svc",
		"port": "80",
	}
	if c.member {
		ks["host"] = c.KubeConf.Cluster.Aicp.HostIp.To4().String()
		ks["port"] = "30880"
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": c.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"db": map[string]interface{}{
			"username": common.PG_AICP,
			"password": iaasKeys.(map[string]string)[common.PG_AICP],
		},
		"config": map[string]interface{}{
			"domain":      c.KubeConf.Cluster.Aicp.DomainConfig.Domain,
			"aiPrefix":    c.KubeConf.Cluster.Aicp.DomainConfig.Ai,
			"sshHost":     c.KubeConf.Cluster.ControlPlaneEndpoint.Address,
			"clusterName": clusterName,
			"billing":     strconv.FormatBool(c.KubeConf.Cluster.Aicp.Billing),
			"ks":          ks,
			"redis": map[string]interface{}{
				"password": iaasKeys.(map[string]string)[common.REDIS_PASSWORD],
			},
			"iaas": map[string]interface{}{
				"hostPrefix":      c.KubeConf.Cluster.Aicp.DomainConfig.Api,
				"protocol":        c.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
				"zone":            c.KubeConf.Cluster.Aicp.Zone,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
			"docker": map[string]interface{}{
				"host":     c.KubeConf.Cluster.Registry.GetHost(),
				"protocol": protocol,
				"username": auth.Username,
				"password": auth.Password,
			},
		},
	}

	helm := HelmOptions{
		Name:      "config-server",
		Namespace: "aicp-system",
		ChartPath: configServerDir,
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
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

	ipPool := []string{}

	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf(" get interfaces failed: %v", err)
	}
	for _, iface := range ifaces {
		// skip interfaces that are not up or have no IP
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			if ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			ipNetStr := &net.IPNet{
				IP:   ip.Mask(ipNet.Mask),
				Mask: ipNet.Mask,
			}
			ipPool = append(ipPool, ipNetStr.String())
		}

	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"toIpBlockExceptCidrs": strings.Join(ipPool, ","),
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "aicp-web-app",
		Namespace: "aicp-system",
		ChartPath: aicpWebAppDir,
		Values:    vals,
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": e.KubeConf.Cluster.Registry.PrivateRegistry,
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": d.KubeConf.Cluster.Registry.PrivateRegistry,
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

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": m.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_AICP,
				"password": iaasKeys.(map[string]string)[common.PG_AICP],
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
			"offlineRepo": o.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_AICP,
				"password": iaasKeys.(map[string]string)[common.PG_AICP],
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
		"basic": map[string]interface{}{
			"repo": v.KubeConf.Cluster.Registry.PrivateRegistry,
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

type EventBusTask struct {
	common.KubeAction
}

func (e *EventBusTask) Execute(runtime connector.Runtime) error {
	eventBusDir := filepath.Join(e.KubeConf.Arg.AicpWorkDir, "charts", "eventbus")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": e.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}

	helm := HelmOptions{
		Name:      "eventbus",
		Namespace: "aicp-system",
		ChartPath: eventBusDir,
		Values:    vals,
	}
	return helm.Install()
}

type ResourceHubTask struct {
	common.KubeAction
}

func (r *ResourceHubTask) Execute(runtime connector.Runtime) error {
	resourceHubDir := filepath.Join(r.KubeConf.Arg.AicpWorkDir, "charts", "resourcehub")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": r.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "resourcehub",
		Namespace: "aicp-system",
		ChartPath: resourceHubDir,
		Values:    vals,
	}
	return helm.Install()
}
