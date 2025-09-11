package aicp

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/prepare"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
)

type DeployOptionalModules struct {
	common.KubeModule
}

func (h *DeployOptionalModules) IsSkip() bool {
	return h.Skip
}
func (h *DeployOptionalModules) Init() {
	h.Name = "DeployOptionalModules"
	h.Desc = "Deploy Optional Modules"

	GpuOperatorTask := task.LocalTask{
		Name: "GpuOperatorTask",
		Desc: "Deploy Gpu Operator Component",
		Prepare: &prepare.PrepareCollection{
			&GpuOperatorPrepare{Type: "nvidia"},
			&HelmIsInstalled{Name: "gpu-operator", Namespace: "gpu-operator", Not: true},
		},
		Action: new(GpuOperatorTask),
		Retry:  0,
	}
	AscendDevicePluginTask := task.LocalTask{
		Name: "AscendDevicePluginTask",
		Desc: "Deploy Ascend Device Plugin Component",
		Prepare: &prepare.PrepareCollection{
			&GpuOperatorPrepare{Type: "ascend"},
			&HelmIsInstalled{Name: "ascend-device-plugin", Namespace: "kube-system", Not: true},
		},
		Action: new(AscendDevicePluginTask),
		Retry:  0,
	}
	AscendExporterTask := task.LocalTask{
		Name: "AscendExporterTask",
		Desc: "Deploy Ascend Exporter Component",
		Prepare: &prepare.PrepareCollection{
			&GpuOperatorPrepare{Type: "ascend"},
			&HelmIsInstalled{Name: "npu-exporter", Namespace: "npu-exporter-system", Not: true},
		},
		Action: new(AscendExporterTask),
		Retry:  0,
	}
	HamiTask := task.LocalTask{
		Name: "HamiTask",
		Desc: "Deploy Hami Component",
		Prepare: &prepare.PrepareCollection{
			&HamiPrepare{},
			&HelmIsInstalled{Name: "hami", Namespace: "hami", Not: true},
		},
		Action: new(HamiTask),
		Retry:  0,
	}
	NetworkOperatorTask := task.LocalTask{
		Name: "NetworkOperatorTask",
		Desc: "Deploy Network Operator Component",
		Prepare: &prepare.PrepareCollection{
			&NetworkOperatorPrepare{},
			&HelmIsInstalled{Name: "network-operator", Namespace: "network-operator", Not: true},
		},
		Action: new(NetworkOperatorTask),
		Retry:  0,
	}

	h.Tasks = []task.Interface{
		&GpuOperatorTask,
		&AscendDevicePluginTask,
		&AscendExporterTask,
		&HamiTask,
		&NetworkOperatorTask,
	}
}
