package aicp

import (
	"path/filepath"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
)

type GpuOperatorTask struct {
	common.KubeAction
}

func (g *GpuOperatorTask) Execute(runtime connector.Runtime) error {
	gpuDir := filepath.Join(g.KubeConf.Arg.AicpWorkDir, "common", "gpu-operator")

	vals := map[string]interface{}{
		"dcgmExporter": map[string]interface{}{
			"serviceMonitor": map[string]interface{}{
				"honorLabels": true,
			},
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"validator": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"operator": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			"initContainer": map[string]interface{}{
				"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
		"driver": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			"manager": map[string]interface{}{
				"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
		"toolkit": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"devicePlugin": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"dcgm": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},

		"gfd": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"migManager": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"vgpuDeviceManager": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"vfioManager": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			"driverManager": map[string]interface{}{
				"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
		"sandboxDevicePlugin": map[string]interface{}{
			"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"node-feature-discovery": map[string]interface{}{
			"image": map[string]interface{}{
				"repository": g.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
	}
	helm := HelmOptions{
		Name:      "gpu-operator",
		Namespace: "gpu-operator",
		ChartPath: gpuDir,
		Values:    vals,
	}
	return helm.Install()
}

type AscendDevicePluginTask struct {
	common.KubeAction
}

func (a *AscendDevicePluginTask) Execute(runtime connector.Runtime) error {
	ascendDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "common", "ascend-device-plugin")

	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "ascend-device-plugin",
		Namespace: "kube-system",
		ChartPath: ascendDir,
		Values:    vals,
	}
	return helm.Install()
}

type AscendExporterTask struct {
	common.KubeAction
}

func (a *AscendExporterTask) Execute(runtime connector.Runtime) error {
	ascendDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "common", "ascend-npu-exporter")

	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "npu-exporter",
		Namespace: "npu-exporter-system",
		ChartPath: ascendDir,
		Values:    vals,
	}
	return helm.Install()
}

type HamiTask struct {
	common.KubeAction
}

func (h *HamiTask) Execute(runtime connector.Runtime) error {
	hamiDir := filepath.Join(h.KubeConf.Arg.AicpWorkDir, "common", "hami")

	vals := map[string]interface{}{
		"imageRepo": h.KubeConf.Cluster.Registry.PrivateRegistry,
	}
	helm := HelmOptions{
		Name:      "hami",
		Namespace: "hami",
		ChartPath: hamiDir,
		Values:    vals,
	}
	return helm.Install()
}

type NetworkOperatorTask struct {
	common.KubeAction
}

func (n *NetworkOperatorTask) Execute(runtime connector.Runtime) error {
	networkOperatorDir := filepath.Join(n.KubeConf.Arg.AicpWorkDir, "common", "network-operator")

	vals := map[string]interface{}{
		"sriov-network-operator": map[string]interface{}{
			"images": map[string]interface{}{
				"repo": n.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
		"operator": map[string]interface{}{
			"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"ofedDriver": map[string]interface{}{
			"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"rdmaSharedDevicePlugin": map[string]interface{}{
			"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"sriovDevicePlugin": map[string]interface{}{
			"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"secondaryNetwork": map[string]interface{}{
			"cniPlugins": map[string]interface{}{
				"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
			},
			"multus": map[string]interface{}{
				"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
			},
			"ipoib": map[string]interface{}{
				"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
			},
			"ipamPlugin": map[string]interface{}{
				"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
		"nicFeatureDiscovery": map[string]interface{}{
			"repository": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "network-operator",
		Namespace: "network-operator",
		ChartPath: networkOperatorDir,
		Values:    vals,
	}
	return helm.Install()
}
