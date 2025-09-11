package aicp

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
)

type DeployIaaSModule struct {
	common.KubeModule
}

func (d *DeployIaaSModule) IsSkip() bool {
	return d.Skip
}

func (d *DeployIaaSModule) Init() {
	d.Name = "DeployIaaSModule"
	d.Desc = "Deploy Global Services "

	ProductManagerServerTask := &task.LocalTask{
		Name:    "ProductManagerServerTask",
		Desc:    "Deploy Product Manager Server Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "product-manager-server", Namespace: "global-system"},
		Action:  new(ProductManagerServerTask),
		Rollback: &DeployFailRollBack{
			Name:      "product-manager-server",
			Namespace: "global-system",
		},
	}
	TeamTask := &task.LocalTask{
		Name:    "TeamTask",
		Desc:    "Deploy Team Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "team", Namespace: "global-system"},
		Action:  new(TeamTask),
		Rollback: &DeployFailRollBack{
			Name:      "team",
			Namespace: "global-system",
		},
	}

	ImaasTask := &task.LocalTask{
		Name:    "ImaasTask",
		Desc:    "Deploy ImaaS Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "imaas", Namespace: "maas-system"},
		Action:  new(ImaasTask),
		Rollback: &DeployFailRollBack{
			Name:      "imaas",
			Namespace: "maas-system",
		},
	}

	AccountTask := &task.LocalTask{
		Name:    "AccountTask",
		Desc:    "Deploy Account Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "account", Namespace: "pitrix"},
		Action:  new(AccountTask),
		Rollback: &DeployFailRollBack{
			Name:      "account",
			Namespace: "pitrix",
		},
	}

	ConsoleTask := &task.LocalTask{
		Name:    "ConsoleTask",
		Desc:    "Deploy Console Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "console", Namespace: "pitrix"},
		Action:  new(ConsoleTask),
		Rollback: &DeployFailRollBack{
			Name:      "console",
			Namespace: "pitrix",
		},
	}
	BossTask := &task.LocalTask{
		Name:    "BossTask",
		Desc:    "Deploy Boss Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "boss", Namespace: "pitrix"},
		Action:  new(BossTask),
		Rollback: &DeployFailRollBack{
			Name:      "boss",
			Namespace: "pitrix",
		},
	}
	ProductTask := &task.LocalTask{
		Name:    "ProductTask",
		Desc:    "Deploy Product Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "product", Namespace: "pitrix"},
		Action:  new(ProductTask),
		Rollback: &DeployFailRollBack{
			Name:      "product",
			Namespace: "pitrix",
		},
	}

	GlueTask := &task.LocalTask{
		Name:    "GlueTask",
		Desc:    "Deploy Glue Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "glue", Namespace: "pitrix"},
		Action:  new(GlueTask),
		Rollback: &DeployFailRollBack{
			Name:      "glue",
			Namespace: "pitrix",
		},
	}
	DocsTask := &task.LocalTask{
		Name:    "DocsTask",
		Desc:    "Deploy Docs Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "docs", Namespace: "pitrix"},
		Action:  new(DocsTask),
		Rollback: &DeployFailRollBack{
			Name:      "docs",
			Namespace: "pitrix",
		},
	}

	BillingTask := &task.LocalTask{
		Name:    "BillingTask",
		Desc:    "Deploy Billing Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "billing", Namespace: "pitrix"},
		Action:  new(BillingTask),
		Rollback: &DeployFailRollBack{
			Name:      "billing",
			Namespace: "pitrix",
		},
	}
	WarehouseTask := &task.LocalTask{
		Name:    "WarehouseTask",
		Desc:    "Deploy Warehouse Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "warehouse", Namespace: "pitrix"},
		Action:  new(WarehouseTask),
	}

	NginxTask := &task.LocalTask{
		Name:    "NginxTask",
		Desc:    "Deploy Nginx Ingress Controller",
		Prepare: &HelmIsInstalled{Not: true, Name: "nginx", Namespace: "pitrix"},
		Action:  new(NginxTask),
		Rollback: &DeployFailRollBack{
			Name:      "nginx",
			Namespace: "pitrix",
		},
	}

	d.Tasks = []task.Interface{
		ProductManagerServerTask,
		TeamTask,
		ImaasTask,
		AccountTask,
		ConsoleTask,
		BossTask,
		ProductTask,
		GlueTask,
		DocsTask,
		BillingTask,
		WarehouseTask,
		NginxTask,
	}
}
