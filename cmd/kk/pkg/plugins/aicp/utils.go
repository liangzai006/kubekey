package aicp

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

type HelmOptions struct {
	Name      string                 // release 名
	Namespace string                 // namespace
	ChartPath string                 // 本地 chart 路径 或 repo 名称
	Values    map[string]interface{} // 自定义 values
	PreHook   func(ctx context.Context, kubeClient kubernetes.Interface) error
	PostHook  func(ctx context.Context, kubeClient kubernetes.Interface, rel *release.Release) error
}

type BaseHelm interface {
	Init() (*action.Configuration, error)
	Install() error
	Templates() (*release.Release, error)
}

func (h *HelmOptions) Init() (*action.Configuration, error) {
	cli := cli.New()
	cli.SetNamespace(h.Namespace)
	cli.KubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	if h.PreHook != nil {
		if err := h.PreHook(ctx, kubeClient); err != nil {
			return err
		}
	}

	if h.Namespace != "" {
		_, err = kubeClient.CoreV1().Namespaces().Get(ctx, h.Namespace, v1.GetOptions{})
		if err != nil && apierrors.IsNotFound(err) {
			_, err = kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: h.Namespace,
				},
			}, v1.CreateOptions{})
			if err != nil {
				return err
			}
		}

	}

	timeout := 300 * time.Second

	i := action.NewInstall(cfg)
	i.ReleaseName = h.Name
	i.Namespace = h.Namespace
	i.Timeout = timeout
	rel, err := i.RunWithContext(ctx, chart, h.Values)
	if err != nil {
		klog.Errorf("install %s failed, %s\n", h.Name, err)
		return err
	}

	if h.PostHook != nil {
		if err := h.PostHook(ctx, kubeClient, rel); err != nil {
			return err
		}
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
	i.Wait = true
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
