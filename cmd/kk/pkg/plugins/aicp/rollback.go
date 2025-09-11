package aicp

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/ending"
)

type DeployFailRollBack struct {
	common.KubeRollback
	Name      string
	Namespace string
}

func (r *DeployFailRollBack) Execute(runtime connector.Runtime, result *ending.ActionResult) error {

	helm := HelmOptions{
		Name:      r.Name,
		Namespace: r.Namespace,
	}

	return helm.Uninstall()
}
