package aicp

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type HelmIsInstalled struct {
	common.KubePrepare
	Not       bool
	Name      string
	Namespace string
}

func (h *HelmIsInstalled) PreCheck(runtime connector.Runtime) (bool, error) {

	if h.Name == "" && h.Namespace == "" {
		return h.Not, nil
	}
	helm := HelmOptions{
		Name:      h.Name,
		Namespace: h.Namespace,
	}

	cfg, err := helm.Init()
	if err != nil {
		return h.Not, err
	}
	get := action.NewGet(cfg)
	helmrelease, err := get.Run(h.Name)
	if err != nil && err != driver.ErrReleaseNotFound {
		return h.Not, err
	}

	if helmrelease.Info.Status == release.StatusFailed {
		return h.Not, nil
	}

	return !h.Not, nil
}

type GpuOperatorPrepare struct {
	common.KubePrepare
	Type string
}

func (g *GpuOperatorPrepare) PreCheck(runtime connector.Runtime) (bool, error) {
	for _, host := range runtime.GetAllHosts() {
		kubeHost := host.(*v1alpha2.KubeHost)
		if kubeHost.GpuType == g.Type {
			return true, nil
		}
	}
	return false, nil

}

type HamiPrepare struct {
	common.KubePrepare
}

func (h *HamiPrepare) PreCheck(runtime connector.Runtime) (bool, error) {
	if h.KubeConf.Arg.Hami {
		return true, nil
	}
	return h.KubeConf.Cluster.Aicp.Hami, nil
}

type NetworkOperatorPrepare struct {
	common.KubePrepare
}

func (n *NetworkOperatorPrepare) PreCheck(runtime connector.Runtime) (bool, error) {
	if n.KubeConf.Arg.Network {
		return true, nil
	}
	return n.KubeConf.Cluster.Aicp.Network, nil
}
