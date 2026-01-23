package aicp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type HelmOptions struct {
	Name       string                 // release 名
	Namespace  string                 // namespace
	ChartPath  string                 // 本地 chart 路径 或 repo 名称
	Values     map[string]interface{} // 自定义 values
	PreHook    func(ctx context.Context, kubeClient kubernetes.Interface) error
	KubeConfig string // kubeconfig 路径
}

type BaseHelm interface {
	Init() (*action.Configuration, error)
	Install() error
	Templates() (*release.Release, error)
}

func (h *HelmOptions) Init() (*action.Configuration, error) {
	cli := cli.New()
	cli.SetNamespace(h.Namespace)
	if h.KubeConfig == "" {
		cli.KubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	} else {
		cli.KubeConfig = h.KubeConfig
	}

	cfg := new(action.Configuration)
	err := cfg.Init(cli.RESTClientGetter(), cli.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		klog.Infof(format, v...)
	})
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func (h *HelmOptions) Install() error {
	cfg, err := h.Init()
	if err != nil {
		return err
	}
	chart, err := loader.Load(h.ChartPath)
	if err != nil {
		klog.Errorf("loading %s chart failed, %s\n", h.ChartPath, err)
		return err
	}
	kubeClient, err := cfg.KubernetesClientSet()
	if err != nil {
		klog.Errorf("get kubernetes client set failed, %s\n", err)
		return err
	}
	timeout := 100 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if h.PreHook != nil {
		if err := h.PreHook(ctx, kubeClient); err != nil {
			return err
		}
	}

	get := action.NewGet(cfg)
	getRelease, err := get.Run(h.Name)
	if err != nil && err != driver.ErrReleaseNotFound {
		return err
	}
	if getRelease == nil {

		i := action.NewInstall(cfg)
		i.ReleaseName = h.Name
		i.Namespace = h.Namespace
		i.CreateNamespace = true
		i.Timeout = timeout
		_, err = i.RunWithContext(ctx, chart, h.Values)
		if err != nil {
			klog.Errorf("install %s failed, %s\n", h.Name, err)
			return err
		}
		// file.WriteString(rel.Manifest)
	} else {
		u := action.NewUpgrade(cfg)
		u.Namespace = h.Namespace
		u.Timeout = timeout
		_, err = u.RunWithContext(ctx, h.Name, chart, h.Values)
		if err != nil {
			klog.Errorf("upgrade %s failed, %s\n", h.Name, err)
			return err
		}

	}

	return nil
}

func (h *HelmOptions) Upgrade() error {
	cfg, err := h.Init()
	if err != nil {
		return err
	}
	upgrade := action.NewUpgrade(cfg)
	upgrade.Namespace = h.Namespace
	upgrade.Timeout = 300 * time.Second

	chart, err := loader.Load(h.ChartPath)
	if err != nil {
		klog.Errorf("loading %s chart failed, %s\n", h.ChartPath, err)
		return err
	}

	_, err = upgrade.Run(h.Name, chart, h.Values)
	if err != nil {
		klog.Errorf("upgrade %s failed, %s\n", h.Name, err)
		return err
	}
	return nil
}

func (h *HelmOptions) Templates() (*release.Release, error) {
	cfg, err := h.Init()
	if err != nil {
		return nil, err
	}

	chart, err := loader.Load(h.ChartPath)
	if err != nil {
		klog.Errorf("loading %s chart failed, %s\n", h.ChartPath, err)
		return nil, err
	}
	i := action.NewInstall(cfg)
	i.ReleaseName = h.Name
	i.Namespace = h.Namespace
	i.DryRun = true

	i.Replace = true // Skip the name check
	i.ClientOnly = true

	rel, err := i.Run(chart, h.Values)
	if err != nil {
		klog.Errorf("templates %s failed, %s\n", h.Name, err)
		return nil, err
	}

	return rel, nil
}

func (h *HelmOptions) Uninstall() error {

	cfg, err := h.Init()
	if err != nil {
		return err
	}
	i := action.NewUninstall(cfg)
	_, err = i.Run(h.Name)

	if err != nil {
		return err
	}
	return nil
}

func (h *HelmOptions) GetHistoryRelease() ([]*release.Release, error) {
	cfg, err := h.Init()
	if err != nil {
		return nil, err
	}

	history := action.NewHistory(cfg)
	history.Max = 10
	releases, err := history.Run(h.Name)
	if err != nil {
		return nil, err
	}
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Version > releases[j].Version
	})
	if len(releases) == 0 {
		return nil, driver.ErrReleaseNotFound
	}
	return releases, nil
}

func FormatBilling(billing bool) int {
	if billing {
		return 1
	}
	return 0
}

func PortForwardToService(kubeConfig string, namespace, serviceName string, servicePort int, localPort int, stopChan, readyChan chan struct{}) error {

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}

	// 获取 Service 的 Endpoints 来找到后端 Pod
	endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("获取 Service Endpoints 失败: %w", err)
	}

	// 从 Endpoints 中找到一个可用的 Pod
	var podName string
	var podPort int32

	for _, subset := range endpoints.Subsets {
		if len(subset.Addresses) > 0 && len(subset.Ports) > 0 {
			// 使用第一个可用的地址
			if subset.Addresses[0].TargetRef != nil && subset.Addresses[0].TargetRef.Kind == "Pod" {
				podName = subset.Addresses[0].TargetRef.Name
			}
			// 找到匹配的端口
			for _, port := range subset.Ports {
				if int(port.Port) == servicePort {
					podPort = port.Port
					break
				}
			}
			if podName != "" && podPort > 0 {
				break
			}
		}
	}

	if podName == "" {
		return fmt.Errorf("Service %s/%s 没有可用的 Pod", namespace, serviceName)
	}

	log.Printf("通过 Service %s/%s 找到 Pod: %s (端口: %d)\n",
		namespace, serviceName, podName, podPort)

	// 构建到 Pod 的 port-forward 请求
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	// 创建 SPDY 传输
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return fmt.Errorf("创建 SPDY transport 失败: %w", err)
	}

	// 创建拨号器
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	// 创建 port-forwarder，使用 Pod 的实际端口
	ports := []string{fmt.Sprintf("%d:%d", localPort, podPort)}
	fw, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("创建 port-forwarder 失败: %w", err)
	}

	// 启动 port-forward
	return fw.ForwardPorts()
}
