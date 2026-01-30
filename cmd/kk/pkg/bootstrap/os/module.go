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

package os

import (
	"path/filepath"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os/templates"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/action"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/prepare"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/kubernetes"
)

type ConfigureOSModule struct {
	common.KubeModule
	Skip bool
}

func (c *ConfigureOSModule) IsSkip() bool {
	return c.Skip
}

func (c *ConfigureOSModule) Init() {
	c.Name = "ConfigureOSModule"
	c.Desc = "Init os dependencies"

	getOSData := &task.RemoteTask{
		Name:     "GetOSData",
		Desc:     "Get OS release",
		Hosts:    c.Runtime.GetAllHosts(),
		Action:   new(GetOSData),
		Parallel: true,
	}

	initOS := &task.RemoteTask{
		Name:     "InitOS",
		Desc:     "Prepare to init OS",
		Hosts:    c.Runtime.GetAllHosts(),
		Action:   new(NodeConfigureOS),
		Parallel: true,
	}

	GenerateOsScript := &task.RemoteTask{
		Name:    "GenerateScript",
		Desc:    "Generate init os script",
		Hosts:   c.Runtime.GetAllHosts(),
		Prepare: &kubernetes.NodeInCluster{Not: true},
		Action: &action.Template{
			Template: templates.InitOsScriptTmpl,
			Dst:      filepath.Join(common.KubeScriptDir, "initOS.sh"),
		},
		Parallel: true,
	}

	ExecOsScript := &task.RemoteTask{
		Name:     "ExecScript",
		Desc:     "Exec init os script",
		Hosts:    c.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(NodeExecScript),
		Parallel: true,
	}

	GenerateHostsScript := &task.RemoteTask{
		Name:  "GenerateHostsScript",
		Desc:  "Generate init hosts script",
		Hosts: c.Runtime.GetAllHosts(),
		Action: &action.Template{
			Template: templates.InitHostsScriptTmpl,
			Dst:      filepath.Join(common.KubeScriptDir, "initHosts.sh"),
			Data: util.Data{
				"Hosts": templates.GenerateHosts(c.Runtime, c.KubeConf),
			},
		},
		Parallel: true,
	}

	ExecHostsScript := &task.RemoteTask{
		Name:     "ExecHostsScript",
		Desc:     "Exec init hosts script",
		Hosts:    c.Runtime.GetAllHosts(),
		Action:   new(NodeExecHostsScript),
		Parallel: true,
	}

	ConfigureNtpServer := &task.RemoteTask{
		Name:     "ConfigureNtpServer",
		Desc:     "configure the ntp server for each node",
		Hosts:    c.Runtime.GetAllHosts(),
		Prepare:  new(NodeConfigureNtpCheck),
		Action:   new(NodeConfigureNtpServer),
		Parallel: true,
	}

	c.Tasks = []task.Interface{
		getOSData,
		initOS,
		GenerateOsScript,
		ExecOsScript,
		GenerateHostsScript,
		ExecHostsScript,
		ConfigureNtpServer,
	}
}

type ClearNodeOSModule struct {
	common.KubeModule
}

func (c *ClearNodeOSModule) Init() {
	c.Name = "ClearNodeOSModule"

	stopKubelet := &task.RemoteTask{
		Name:     "StopKubelet",
		Desc:     "Stop Kubelet",
		Hosts:    c.Runtime.GetHostsByRole(common.Worker),
		Prepare:  new(DeleteNode),
		Action:   new(StopKubelet),
		Parallel: true,
	}

	resetNetworkConfig := &task.RemoteTask{
		Name:     "ResetNetworkConfig",
		Desc:     "Reset os network config",
		Hosts:    c.Runtime.GetHostsByRole(common.Worker),
		Prepare:  new(DeleteNode),
		Action:   new(ResetNetworkConfig),
		Parallel: true,
	}

	removeFiles := &task.RemoteTask{
		Name:     "RemoveFiles",
		Desc:     "Remove node files",
		Hosts:    c.Runtime.GetHostsByRole(common.Worker),
		Prepare:  new(DeleteNode),
		Action:   new(RemoveNodeFiles),
		Parallel: true,
	}

	daemonReload := &task.RemoteTask{
		Name:     "DaemonReload",
		Desc:     "Systemd daemon reload",
		Hosts:    c.Runtime.GetHostsByRole(common.Worker),
		Prepare:  new(DeleteNode),
		Action:   new(DaemonReload),
		Parallel: true,
	}

	c.Tasks = []task.Interface{
		stopKubelet,
		resetNetworkConfig,
		removeFiles,
		daemonReload,
	}
}

type ClearOSEnvironmentModule struct {
	common.KubeModule
}

func (c *ClearOSEnvironmentModule) Init() {
	c.Name = "ClearOSModule"

	resetNetworkConfig := &task.RemoteTask{
		Name:     "ResetNetworkConfig",
		Desc:     "Reset os network config",
		Hosts:    c.Runtime.GetHostsByRole(common.K8s),
		Action:   new(ResetNetworkConfig),
		Parallel: true,
	}

	uninstallETCD := &task.RemoteTask{
		Name:  "UninstallETCD",
		Desc:  "Uninstall etcd",
		Hosts: c.Runtime.GetHostsByRole(common.ETCD),
		Prepare: &prepare.PrepareCollection{
			new(EtcdTypeIsKubeKey),
		},
		Action:   new(UninstallETCD),
		Parallel: true,
	}

	removeFiles := &task.RemoteTask{
		Name:     "RemoveFiles",
		Desc:     "Remove cluster files",
		Hosts:    c.Runtime.GetHostsByRole(common.K8s),
		Action:   new(RemoveFiles),
		Parallel: true,
	}

	daemonReload := &task.RemoteTask{
		Name:     "DaemonReload",
		Desc:     "Systemd daemon reload",
		Hosts:    c.Runtime.GetHostsByRole(common.K8s),
		Action:   new(DaemonReload),
		Parallel: true,
	}

	c.Tasks = []task.Interface{
		resetNetworkConfig,
		uninstallETCD,
		removeFiles,
		daemonReload,
	}
}

type RepositoryOnlineModule struct {
	common.KubeModule
	Skip bool
}

func (r *RepositoryOnlineModule) IsSkip() bool {
	return r.Skip
}

func (r *RepositoryOnlineModule) Init() {
	r.Name = "RepositoryOnlineModule"

	getOSData := &task.RemoteTask{
		Name:     "GetOSData",
		Desc:     "Get OS release",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(GetOSData),
		Parallel: true,
	}

	newRepo := &task.RemoteTask{
		Name:     "NewRepoClient",
		Desc:     "New repository client",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(NewRepoClient),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
	}

	preInstall := &task.RemoteTask{
		Name:     "InstallPackage",
		Desc:     "Install packages",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(PreInstallPackage),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	customAfterPreInstallScriptTask := &task.RemoteTask{
		Name:     "CustomAfterPreInstallScriptTask",
		Desc:     "Custom After PreInstall Script Task",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(CustomAfterPreInstall),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
	}

	install := &task.RemoteTask{
		Name:     "InstallPackage",
		Desc:     "Install packages",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(InstallPackage),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
	}

	postInstall := &task.RemoteTask{
		Name:     "InstallPackage",
		Desc:     "Install packages",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(PostInstallPackage),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	customAfterPostInstallScriptTask := &task.RemoteTask{
		Name:     "CustomAfterPostInstallScriptTask",
		Desc:     "Custom After PostInstall Script Task",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(CustomAfterPostInstall),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    1,
	}

	r.Tasks = []task.Interface{
		getOSData,
		newRepo,
		preInstall,
		customAfterPreInstallScriptTask,
		install,
		postInstall,
		customAfterPostInstallScriptTask,
	}
}

type RepositoryModule struct {
	common.KubeModule
	Skip bool
}

func (r *RepositoryModule) IsSkip() bool {
	return r.Skip
}

func (r *RepositoryModule) Init() {
	r.Name = "RepositoryModule"
	r.Desc = "Install local repository"

	getOSData := &task.RemoteTask{
		Name:     "GetOSData",
		Desc:     "Get OS release",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(GetOSData),
		Parallel: false,
	}

	getLocalOSData := &task.LocalTask{
		Name:   "GetLocalOSData",
		Desc:   "Get Local OS release",
		Action: new(GetLocalOSData),
	}
	sync := &task.LocalTask{
		Name:     "SyncRepositoryISOFile",
		Desc:     "Sync repository iso file to tmp path",
		Action:   new(SyncRepositoryLocalFiles),
		Rollback: nil,
		Retry:    2,
	}

	mount := &task.LocalTask{
		Name:   "MountISO",
		Desc:   "Mount iso file",
		Action: new(LocalMountISO),
		Retry:  1,
	}

	newServer := &task.LocalTask{
		Name:   "NewServer",
		Desc:   "New repository server ",
		Action: new(NewRepoServer),
		Retry:  1,
	}

	newRepo := &task.RemoteTask{
		Name:     "NewRepoClient",
		Desc:     "New repository client",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(NewRepoClient),
		Parallel: true,
		Retry:    1,
		Rollback: new(RollbackUmount),
	}

	backup := &task.RemoteTask{
		Name:     "BackupOriginalRepository",
		Desc:     "Backup original repository",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(BackupOriginalRepository),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverBackupSuccessNode),
	}

	add := &task.RemoteTask{
		Name:     "AddLocalRepository",
		Desc:     "Add local repository",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(AddLocalRepository),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	preInstall := &task.RemoteTask{
		Name:     "InstallPrePackage",
		Desc:     "InstallPrepackages",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(PreInstallPackage),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	customAfterPreInstallScriptTask := &task.RemoteTask{
		Name:     "CustomAfterPreInstallScriptTask",
		Desc:     "Custom After PreInstall Script Task",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(CustomAfterPreInstall),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	install := &task.RemoteTask{
		Name:     "InstallPackage",
		Desc:     "Install packages",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(InstallPackage),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	postInstall := &task.RemoteTask{
		Name:     "InstallPostPackage",
		Desc:     "InstallPostPackages",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(PostInstallPackage),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	customAfterPostInstallScriptTask := &task.RemoteTask{
		Name:     "CustomAfterPostInstallScriptTask",
		Desc:     "Custom After PostInstall Script Task",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(CustomAfterPostInstall),
		Parallel: true,
		Retry:    1,
		Rollback: new(RecoverRepository),
	}

	reset := &task.RemoteTask{
		Name:     "ResetRepository",
		Desc:     "Reset repository to the original repository",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Action:   new(ResetRepository),
		Parallel: true,
		Retry:    1,
	}

	umount := &task.LocalTask{
		Name:   "UmountISO",
		Desc:   "Umount ISO file",
		Action: new(LocalUmountISO),
	}

	r.Tasks = []task.Interface{
		getOSData,
		getLocalOSData,
		sync,
		mount,
		newServer,
		newRepo,
		backup,
		add,
		preInstall,
		customAfterPreInstallScriptTask,
		install,
		postInstall,
		customAfterPostInstallScriptTask,
		reset,
		umount,
	}
}

type AicpDirModule struct {
	common.KubeModule
	Skip bool
}

func (r *AicpDirModule) IsSkip() bool {
	return r.Skip
}

func (r *AicpDirModule) Init() {
	r.Name = "AicpDirModule"
	r.Desc = "Config aicp dir"

	configRootDir := &task.RemoteTask{
		Name:     "ConfigAicpRootDir",
		Desc:     "Config Aicp docker root dir",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(ConfigAicpRootDir),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    2,
	}

	configDataDir := &task.RemoteTask{
		Name:     "ConfigAicpDataDir",
		Desc:     "Config Aicp data dir",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(ConfigAicpDataDir),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    2,
	}
	configLongHornVolumeDir := &task.RemoteTask{
		Name:     "ConfigLongHornVolumeDir",
		Desc:     "Config Longhorn volume dir",
		Hosts:    r.Runtime.GetAllHosts(),
		Action:   new(ConfigLongHornVolumeDir),
		Prepare:  &kubernetes.NodeInCluster{Not: true},
		Parallel: true,
		Retry:    2,
	}

	r.Tasks = []task.Interface{
		configRootDir,
	}
	if r.KubeConf.Cluster.Aicp.Storage == common.Longhorn {
		r.Tasks = append(r.Tasks, configLongHornVolumeDir)
	} else {
		r.Tasks = append(r.Tasks, configDataDir)
	}
}

type FreePasswdModule struct {
	common.KubeModule
	Skip bool
}

func (r *FreePasswdModule) IsSkip() bool {
	return r.Skip
}

func (r *FreePasswdModule) Init() {
	r.Name = "SSHFreePasswdModule"
	r.Desc = "SSH Node Free passwd"

	generateFreePasswd := &task.RemoteTask{
		Name:     "GenerateFreePasswd",
		Desc:     "Generate Free passwd",
		Hosts:    r.Runtime.GetAllHosts(),
		Prepare:  &SShPublickeyPrepare{Not: true},
		Action:   new(GenerateSSHKey),
		Parallel: false,
	}
	getSSHPublicKey := &task.RemoteTask{
		Name:    "GetSSHPublicKey",
		Desc:    "Get SSH public key",
		Prepare: &SShPublickeyPrepare{Not: false},
		Hosts:   r.Runtime.GetAllHosts(),
		Action:  new(GetSSHPublicKey),
	}
	copyMasterSSHKey := &task.RemoteTask{
		Name:   "CopyMasterSSHKey",
		Desc:   "Copy Master SSH key",
		Hosts:  r.Runtime.GetHostsByRole(common.Master),
		Action: new(CopyMasterSSHKey),
	}
	copyWorkerSSHKey := &task.RemoteTask{
		Name:   "CopyWorkerSSHKey",
		Desc:   "Copy Worker SSH key",
		Hosts:  r.Runtime.GetHostsByRole(common.K8s),
		Action: new(CopyWorkerSSHKey),
	}
	execSSHKey := &task.RemoteTask{
		Name:   "ExecSSHKey",
		Desc:   "Exec SSH key",
		Hosts:  r.Runtime.GetAllHosts(),
		Action: new(ExecSSHKey),
	}

	r.Tasks = []task.Interface{
		generateFreePasswd,
		getSSHPublicKey,
		copyMasterSSHKey,
		copyWorkerSSHKey,
		execSSHKey,
	}
}
