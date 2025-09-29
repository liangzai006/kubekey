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
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubesphere"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/loadbalancer"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/aicp"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/dns"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/network"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/storage"
	reg "github.com/kubesphere/kubekey/v3/cmd/kk/pkg/registry"
	"github.com/modood/table"
)

func NewDeployPipeline(runtime *common.KubeRuntime) error {
	noArtifact := runtime.Arg.Artifact == ""
	skipPushImages := noArtifact || (!noArtifact && runtime.Cluster.Registry.PrivateRegistry == "")

	m := []module.Module{
		&precheck.ClusterConfigPreCheckModule{},
		&precheck.GreetingsModule{},
		&customscripts.CustomScriptsModule{Phase: "PreInstall", Scripts: runtime.Cluster.System.PreInstall},
		&precheck.NodePreCheckModule{},
		&confirm.InstallConfirmModule{},
		&os.FreePasswdModule{Skip: !runtime.Arg.FreePasswd},
		&artifact.UnArchiveModule{Skip: noArtifact || runtime.Arg.SkipCheckMd5 || runtime.IsStepSkip("initOs")},
		&kubernetes.StatusModule{},
		&os.RepositoryModule{Skip: noArtifact && !runtime.Arg.InstallPackages || runtime.IsStepSkip("initOs")},
		&os.ConfigureOSModule{Skip: runtime.Cluster.System.SkipConfigureOS},
		&os.AicpDirModule{Skip: runtime.IsStepSkip("initOs")},
		/** deploy registry **/
		&binaries.RegistryPackageModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},
		&registry.RegistryCertsModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},
		&registry.InstallRegistryModule{Skip: runtime.Cluster.Registry.PrivateRegistry == ""},
		&images.CopyImagesToRegistryModule{Skip: skipPushImages},
		/** deploy kubernetes **/
		&binaries.NodeBinariesModule{Skip: runtime.IsStepSkip("createCluster")},
		&filesystem.ChownWorkDirModule{},
		&container.InstallContainerModule{Skip: runtime.IsStepSkip("createCluster")},
		&container.InstallCriDockerdModule{Skip: runtime.Cluster.Kubernetes.ContainerManager != "docker" || runtime.IsStepSkip("createCluster")},
		&images.PullModule{Skip: runtime.Arg.SkipPullImages || runtime.IsStepSkip("createCluster")},
		&etcd.PreCheckModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.IsStepSkip("createCluster")},
		&etcd.CertsModule{Skip: runtime.IsStepSkip("createCluster")},
		&etcd.InstallETCDBinaryModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.IsStepSkip("createCluster")},
		&etcd.ConfigureModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.IsStepSkip("createCluster")},
		&etcd.BackupModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.IsStepSkip("createCluster")},
		&kubernetes.InstallKubeBinariesModule{Skip: runtime.IsStepSkip("createCluster")},
		// init kubeVip on first master
		&loadbalancer.KubevipModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabledVip() || runtime.IsStepSkip("createCluster")},
		&kubernetes.InitKubernetesModule{Skip: runtime.IsStepSkip("createCluster")},
		&dns.ClusterDNSModule{Skip: runtime.IsStepSkip("createCluster")},
		&kubernetes.StatusModule{Skip: runtime.IsStepSkip("createCluster")},
		&kubernetes.JoinNodesModule{Skip: runtime.IsStepSkip("createCluster")},
		// deploy kubeVip on other masters
		&loadbalancer.KubevipModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabledVip() || runtime.IsStepSkip("createCluster")},
		&loadbalancer.HaproxyModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabled() || runtime.IsStepSkip("createCluster")},
		&network.DeployNetworkPluginModule{Skip: runtime.IsStepSkip("createCluster")},
		&kubernetes.ConfigureKubernetesModule{Skip: runtime.IsStepSkip("createCluster")},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !runtime.Cluster.Kubernetes.EnableAutoRenewCerts() || runtime.IsStepSkip("createCluster")},
		&kubernetes.SecurityEnhancementModule{Skip: !runtime.Arg.SecurityEnhancement || runtime.IsStepSkip("createCluster")},
		&kubernetes.SaveKubeConfigModule{Skip: runtime.IsStepSkip("createCluster")},
		&plugins.DeployPluginsModule{Skip: runtime.IsStepSkip("createCluster")},
		&addons.AddonsModule{Skip: runtime.IsStepSkip("createCluster")},
		&customscripts.CustomScriptsModule{Phase: "PostInstall", Scripts: runtime.Cluster.System.PostInstall},

		// deploy storage volume zfs
		&storage.DeployStorageVolumeModule{},
		&kubesphere.DeployKsCoreModule{},
		&aicp.DeployAicpServiceModule{},
		&aicp.DeployIaaSModule{},
		&aicp.DeployOptionalModules{},
	}

	p := pipeline.Pipeline{
		Name:    "CoresHubDeployPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}

	PrintDeployInfo(runtime)

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

// console   10.23.0.2 console.core.com   admin Zhu88jie!
type PrintDeployInfoResult struct {
	Name     string `table:"name"`
	Ip       string `table:"ip"`
	Domain   string `table:"domain"`
	Username string `table:"username"`
	Password string `table:"password"`
}

func PrintDeployInfo(runtime *common.KubeRuntime) {

	auths := reg.DockerRegistryAuthEntries(runtime.Cluster.Registry.Auths)

	var username, password = "admin", "Harbor12345"

	if auth, ok := auths[runtime.Cluster.Registry.GetHost()]; ok {
		username = auth.Username
		password = auth.Password
	}

	var registryId string
	registryHosts := runtime.GetHostsByRole(common.Registry)
	if len(registryHosts) > 0 {
		registryId = registryHosts[0].GetInternalAddress()
	}

	results := []PrintDeployInfoResult{
		{
			Name:     "console",
			Ip:       runtime.Cluster.ControlPlaneEndpoint.Address,
			Domain:   fmt.Sprintf("console.%s", runtime.Cluster.Aicp.Domain),
			Username: fmt.Sprintf("admin@%s", runtime.Cluster.Aicp.Domain),
			Password: "需boss重置密码",
		},
		{
			Name:     "boss",
			Ip:       runtime.Cluster.ControlPlaneEndpoint.Address,
			Domain:   fmt.Sprintf("boss.%s", runtime.Cluster.Aicp.Domain),
			Username: fmt.Sprintf("boss@%s", runtime.Cluster.Aicp.Domain),
			Password: "zhu88jie",
		},
		{
			Name:     "cadmin",
			Ip:       runtime.Cluster.ControlPlaneEndpoint.Address,
			Domain:   fmt.Sprintf("cadmin.%s", runtime.Cluster.Aicp.Domain),
			Username: "admin",
			Password: "zhu88jie",
		},
		{
			Name:     "kse",
			Domain:   fmt.Sprintf("%s:30880", runtime.Cluster.ControlPlaneEndpoint.Address),
			Username: "admin",
			Password: "P@88w0rd",
		},
		{
			Name:     "harbor",
			Ip:       registryId,
			Domain:   runtime.Cluster.Registry.GetHost(),
			Username: username,
			Password: password,
		},
	}
	fmt.Println()
	fmt.Println("部署完成。集群服务的相关信息如下：")
	table.OutputA(results)
}
