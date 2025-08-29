/*
 Copyright 2021 The KubeSphere Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package pipelines

import (
	"fmt"

	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/addons"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/artifact"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/binaries"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/confirm"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/customscripts"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/registry"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/certs"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/container"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/module"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/pipeline"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/etcd"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/filesystem"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/images"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/loadbalancer"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/dns"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/network"
)

func NewDeployPipeline(runtime *common.KubeRuntime) error {
	noArtifact := runtime.Arg.Artifact == ""
	skipPushImages := noArtifact || (!noArtifact && runtime.Cluster.Registry.PrivateRegistry == "")

	m := []module.Module{
		&precheck.GreetingsModule{},
		&customscripts.CustomScriptsModule{Phase: "PreInstall", Scripts: runtime.Cluster.System.PreInstall},
		&precheck.NodePreCheckModule{},
		&confirm.InstallConfirmModule{},
		&artifact.UnArchiveModule{Skip: noArtifact},
		&kubernetes.StatusModule{},
		&os.RepositoryModule{Skip: noArtifact && !runtime.Arg.InstallPackages},
		&binaries.NodeBinariesModule{},
		&binaries.RegistryPackageModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},
		&os.ConfigureOSModule{Skip: runtime.Cluster.System.SkipConfigureOS},
		&os.AicpDirModule{Skip: !runtime.Arg.IsAicpCluster},
		&registry.RegistryCertsModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},
		&registry.InstallRegistryModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},

		&filesystem.ChownWorkDirModule{},
		&container.InstallContainerModule{},
		&container.InstallCriDockerdModule{Skip: runtime.Cluster.Kubernetes.ContainerManager != "docker"},
		&images.CopyImagesToRegistryModule{Skip: skipPushImages},
		&images.PullModule{Skip: runtime.Arg.SkipPullImages},
		&etcd.PreCheckModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.CertsModule{},
		&etcd.InstallETCDBinaryModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.ConfigureModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.BackupModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&kubernetes.InstallKubeBinariesModule{},
		// init kubeVip on first master
		&loadbalancer.KubevipModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabledVip()},
		&kubernetes.InitKubernetesModule{},
		&dns.ClusterDNSModule{},
		&kubernetes.StatusModule{},
		&kubernetes.JoinNodesModule{},
		// deploy kubeVip on other masters
		&loadbalancer.KubevipModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabledVip()},
		&loadbalancer.HaproxyModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabled()},
		&network.DeployNetworkPluginModule{},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !runtime.Cluster.Kubernetes.EnableAutoRenewCerts()},
		&kubernetes.SecurityEnhancementModule{Skip: !runtime.Arg.SecurityEnhancement},
		&kubernetes.SaveKubeConfigModule{},
		&plugins.DeployPluginsModule{},
		&addons.AddonsModule{},
		&customscripts.CustomScriptsModule{Phase: "PostInstall", Scripts: runtime.Cluster.System.PostInstall},
	}

	p := pipeline.Pipeline{
		Name:    "CoresHubDeployPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}

	fmt.Print(`CoresHub deployment is complete.

Please check the result using the command:

	kubectl get pod -A
`)

	return nil
}

func CoresHubDeploy(args common.Argument, downloadCmd string) error {
	args.DownloadCommand = func(path, url string) string {
		// this is an extension point for downloading tools, for example users can set the timeout, proxy or retry under
		// some poor network environment. Or users even can choose another cli, it might be wget.
		// perhaps we should have a build-in download function instead of totally rely on the external one
		return fmt.Sprintf(downloadCmd, path, url)
	}

	var loaderType string
	if args.FilePath != "" {
		loaderType = common.File
	} else {
		loaderType = common.AllInOne
	}

	runtime, err := common.NewKubeRuntime(loaderType, args)
	if err != nil {
		return err
	}

	if err := NewDeployPipeline(runtime); err != nil {
		return err
	}

	return nil
}
