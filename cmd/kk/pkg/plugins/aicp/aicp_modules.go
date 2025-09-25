package aicp

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
)

type DeployAicpServiceModule struct {
	common.KubeModule
}

func (h *DeployAicpServiceModule) IsSkip() bool {
	return h.Skip
}

func (h *DeployAicpServiceModule) Init() {
	h.Name = "DeployAicpServiceModule"
	h.Desc = "Deploy Aicp Service"

	aicpStorageTask := &task.LocalTask{
		Name:    "AicpStorageTask",
		Desc:    "Deploy Aicp Storage Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "aicp-storage", Namespace: "aicp-storage"},
		Action:  new(AicpStorageTask),
		Rollback: &DeployFailRollBack{
			Name:      "aicp-storage",
			Namespace: "aicp-storage",
		},
	}

	certManagerTask := &task.LocalTask{
		Name:    "CertManagerTask",
		Desc:    "Deploy CertManager Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "cert-manager", Namespace: "cert-manager"},
		Action:  new(CertManagerTask),
		Rollback: &DeployFailRollBack{
			Name:      "cert-manager",
			Namespace: "cert-manager",
		},
	}
	istioTask := &task.LocalTask{
		Name:    "IstioTask",
		Desc:    "Deploy Istio Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "istiod", Namespace: "istio-system"},
		Action:  new(IstioTask),
		Rollback: &DeployFailRollBack{
			Name:      "istiod",
			Namespace: "istio-system",
		},
	}

	ClusterLocalGatewayTask := &task.LocalTask{
		Name:    "ClusterLocalGatewayTask",
		Desc:    "Deploy Cluster Local Gateway",
		Prepare: &HelmIsInstalled{Not: true, Name: "cluster-local-gateway", Namespace: "istio-system"},
		Action:  new(ClusterLocalGatewayTask),
		Rollback: &DeployFailRollBack{
			Name:      "cluster-local-gateway",
			Namespace: "istio-system",
		},
	}

	KubeflowTask := &task.LocalTask{
		Name:    "KubeflowTask",
		Desc:    "Deploy Kubeflow Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "kubeflow", Namespace: "kubeflow"},
		Action:  new(KubeflowTask),
		Rollback: &DeployFailRollBack{
			Name:      "kubeflow",
			Namespace: "kubeflow",
		},
	}

	AuthServerTask := &task.LocalTask{
		Name:    "AuthServerTask",
		Desc:    "Deploy Auth Server Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "auth-server", Namespace: "istio-system"},
		Action:  new(AuthServerTask),
		Rollback: &DeployFailRollBack{
			Name:      "auth-server",
			Namespace: "istio-system",
		},
	}
	PodDefaultsTask := &task.LocalTask{
		Name:    "PodDefaultsTask",
		Desc:    "Deploy Pod Defaults Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "poddefaults", Namespace: "kubeflow"},
		Action:  new(PodDefaultsTask),
		Rollback: &DeployFailRollBack{
			Name:      "poddefaults",
			Namespace: "kubeflow",
		},
	}

	NotebookControllerTask := &task.LocalTask{
		Name:    "NotebookControllerTask",
		Desc:    "Deploy Notebook Controller Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "notebook-controller", Namespace: "kubeflow"},
		Action:  new(NotebookControllerTask),
		Rollback: &DeployFailRollBack{
			Name:      "notebook-controller",
			Namespace: "kubeflow",
		},
	}

	ProfilesTask := &task.LocalTask{
		Name:    "ProfilesTask",
		Desc:    "Deploy Profiles Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "profiles", Namespace: "kubeflow"},
		Action:  new(ProfilesTask),
		Rollback: &DeployFailRollBack{
			Name:      "profiles",
			Namespace: "kubeflow",
		},
	}
	TensorboardControllerTask := &task.LocalTask{
		Name:    "TensorboardControllerTask",
		Desc:    "Deploy Tensorboard Controller Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "tensorboard-controller", Namespace: "kubeflow"},
		Action:  new(TensorboardControllerTask),
		Rollback: &DeployFailRollBack{
			Name:      "tensorboard-controller",
			Namespace: "kubeflow",
		},
	}

	TrainingOperatorTask := &task.LocalTask{
		Name:    "TrainingOperatorTask",
		Desc:    "Deploy Training Operator Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "training-operator", Namespace: "kubeflow"},
		Action:  new(TrainingOperatorTask),
		Rollback: &DeployFailRollBack{
			Name:      "training-operator",
			Namespace: "kubeflow",
		},
	}

	AicpWebAppTask := &task.LocalTask{
		Name:    "AicpWebAppTask",
		Desc:    "Deploy Aicp WebApp Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "aicp-web-app", Namespace: "aicp-system"},
		Action:  new(AicpWebAppTask),
		Rollback: &DeployFailRollBack{
			Name:      "aicp-web-app",
			Namespace: "aicp-system",
		},
	}
	ImagebuilderTask := &task.LocalTask{
		Name:    "ImagebuilderTask",
		Desc:    "Deploy Imagebuilder Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "imagebuilder", Namespace: "aicp-system"},
		Action:  new(ImagebuilderTask),
		Rollback: &DeployFailRollBack{
			Name:      "imagebuilder",
			Namespace: "aicp-system",
		},
	}

	EpfsTask := &task.LocalTask{
		Name:    "EpfsTask",
		Desc:    "Deploy EPFS Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "epfs", Namespace: "aicp-system"},
		Action:  new(EpfsTask),
		Rollback: &DeployFailRollBack{
			Name:      "epfs",
			Namespace: "aicp-system",
		},
	}

	PushServerTask := &task.LocalTask{
		Name:    "PushServerTask",
		Desc:    "Push Server Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "push-server", Namespace: "aicp-system"},
		Action:  new(PushServerTask),
		Rollback: &DeployFailRollBack{
			Name:      "push-server",
			Namespace: "aicp-system",
		},
	}

	DockerApiServerTask := &task.LocalTask{
		Name:    "DockerApiServerTask",
		Desc:    "Deploy Docker Api Server Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "docker-api-server", Namespace: "aicp-system"},
		Action:  new(DockerApiServerTask),
		Rollback: &DeployFailRollBack{
			Name:      "docker-api-server",
			Namespace: "aicp-system",
		},
	}

	ResourceProxyTask := &task.LocalTask{
		Name:    "ResourceProxyTask",
		Desc:    "Deploy Resource Proxy Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "resource-proxy", Namespace: "aicp-resource"},
		Action:  new(ResourceProxyTask),
		Rollback: &DeployFailRollBack{
			Name:      "resource-proxy",
			Namespace: "aicp-resource",
		},
	}

	PrometheusBlackboxExporterTask := &task.LocalTask{
		Name:    "PrometheusBlackboxExporterTask",
		Desc:    "Deploy Prometheus Blackbox Exporter Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "prometheus-blackbox-exporter", Namespace: "kubesphere-monitoring-system"},
		Action:  new(PrometheusBlackboxExporterTask),
		Rollback: &DeployFailRollBack{
			Name:      "prometheus-blackbox-exporter",
			Namespace: "kubesphere-monitoring-system",
		},
	}
	MaasTask := &task.LocalTask{
		Name:    "MaasTask",
		Desc:    "Deploy Maas Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "maas", Namespace: "maas-system"},
		Action:  new(MaasTask),
		Rollback: &DeployFailRollBack{
			Name:      "maas",
			Namespace: "maas-system",
		},
	}
	OperationTask := &task.LocalTask{
		Name:    "OperationTask",
		Desc:    "Deploy Operation Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "operation", Namespace: "aicp-system"},
		Action:  new(OperationTask),
		Rollback: &DeployFailRollBack{
			Name:      "operation",
			Namespace: "aicp-system",
		},
	}

	LwsTask := &task.LocalTask{
		Name:    "LwsTask",
		Desc:    "Deploy Lws Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "lws", Namespace: "lws-system"},
		Action:  new(LwsTask),
		Rollback: &DeployFailRollBack{
			Name:      "lws",
			Namespace: "lws-system",
		},
	}

	VolcanoTask := &task.LocalTask{
		Name:    "VolcanoTask",
		Desc:    "Deploy Volcano Component",
		Prepare: &HelmIsInstalled{Not: true, Name: "volcano", Namespace: "volcano-system"},
		Action:  new(VolcanoTask),
		Rollback: &DeployFailRollBack{
			Name:      "volcano",
			Namespace: "volcano-system",
		},
	}

	h.Tasks = []task.Interface{
		aicpStorageTask,
		certManagerTask,
		istioTask,
		ClusterLocalGatewayTask,
		KubeflowTask,
		AuthServerTask,
		PodDefaultsTask,
		NotebookControllerTask,
		ProfilesTask,
		TensorboardControllerTask,
		VolcanoTask,
		TrainingOperatorTask,
		AicpWebAppTask,
		ImagebuilderTask,
		EpfsTask,
		PushServerTask,
		DockerApiServerTask,
		ResourceProxyTask,
		PrometheusBlackboxExporterTask,
		MaasTask,
		OperationTask,
		LwsTask,
	}
}
