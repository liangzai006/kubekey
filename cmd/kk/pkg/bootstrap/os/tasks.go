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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	kubekeyv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/os/repository"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/utils"
	"github.com/kubesphere/kubekey/v3/util/osrelease"
)

type NodeConfigureOS struct {
	common.KubeAction
}

func (n *NodeConfigureOS) Execute(runtime connector.Runtime) error {

	host := runtime.RemoteHost()
	if err := addUsers(runtime, host); err != nil {
		return errors.Wrap(errors.WithStack(err), "Failed to add users")
	}

	if err := createDirectories(runtime, host); err != nil {
		return err
	}

	if err := utils.ResetTmpDir(runtime); err != nil {
		return err
	}

	_, err1 := runtime.GetRunner().SudoCmd(fmt.Sprintf("hostnamectl set-hostname %s && sed -i '/^127.0.1.1/s/.*/127.0.1.1      %s/g' /etc/hosts", host.GetName(), host.GetName()), false)
	if err1 != nil {
		return errors.Wrap(errors.WithStack(err1), "Failed to override hostname")
	}

	return nil
}

func addUsers(runtime connector.Runtime, node connector.Host) error {
	if _, err := runtime.GetRunner().SudoCmd("useradd -M -c 'Kubernetes user' -s /sbin/nologin -r kube || :", false); err != nil {
		return err
	}

	if node.IsRole(common.ETCD) {
		if _, err := runtime.GetRunner().SudoCmd("useradd -M -c 'Etcd user' -s /sbin/nologin -r etcd || :", false); err != nil {
			return err
		}
	}

	return nil
}

func createDirectories(runtime connector.Runtime, node connector.Host) error {
	dirs := []string{
		common.BinDir,
		common.KubeConfigDir,
		common.KubeCertDir,
		common.KubeManifestDir,
		common.KubeScriptDir,
		common.KubeletFlexvolumesPluginsDir,
	}

	for _, dir := range dirs {
		if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s", dir), false); err != nil {
			return err
		}
		if dir == common.KubeletFlexvolumesPluginsDir {
			if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chown kube -R %s", "/usr/libexec/kubernetes"), false); err != nil {
				return err
			}
		} else {
			if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chown kube -R %s", dir), false); err != nil {
				return err
			}
		}
	}

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s && chown kube -R %s", "/etc/cni/net.d", "/etc/cni"), false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s && chown kube -R %s", "/opt/cni/bin", "/opt/cni"), false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s && chown kube -R %s", "/var/lib/calico", "/var/lib/calico"), false); err != nil {
		return err
	}

	if node.IsRole(common.ETCD) {
		if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s && chown etcd -R %s", "/var/lib/etcd", "/var/lib/etcd"), false); err != nil {
			return err
		}
	}

	return nil
}

type NodeExecScript struct {
	common.KubeAction
}

func (n *NodeExecScript) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chmod +x %s/initOS.sh", common.KubeScriptDir), false); err != nil {
		return errors.Wrap(errors.WithStack(err), "Failed to chmod +x init os script")
	}

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("%s/initOS.sh", common.KubeScriptDir), true); err != nil {
		return errors.Wrap(errors.WithStack(err), "Failed to configure operating system")
	}
	return nil
}

var (
	etcdFiles = []string{
		"/usr/local/bin/etcd",
		"/etc/ssl/etcd",
		"/var/lib/etcd/*",
		"/etc/etcd.env",
	}
	clusterFiles = []string{
		"/etc/kubernetes",
		"/etc/systemd/system/etcd.service",
		"/etc/systemd/system/backup-etcd.service",
		"/etc/systemd/system/backup-etcd.timer",
		"/var/log/calico",
		"/etc/cni",
		"/var/log/pods/",
		"/var/lib/cni",
		"/var/lib/calico",
		"/var/lib/kubelet/*",
		"/run/calico",
		"/run/flannel",
		"/etc/flannel",
		"/var/openebs",
		"/etc/systemd/system/kubelet.service",
		"/etc/systemd/system/kubelet.service.d",
		"/usr/local/bin/kubelet",
		"/usr/local/bin/kubeadm",
		"/usr/bin/kubelet",
		"/var/lib/rook",
		"/tmp/kubekey",
		"/etc/kubekey",
	}

	networkResetCmds = []string{
		"iptables -F",
		"iptables -X",
		"iptables -F -t nat",
		"iptables -X -t nat",
		"ipvsadm -C",
		"ip link del kube-ipvs0",
		"ip link del nodelocaldns",
		"ip link del cni0",
		"ip link del flannel.1",
		"ip link del flannel-v6.1",
		"ip link del flannel-wg",
		"ip link del flannel-wg-v6",
		"ip link del cilium_host",
		"ip link del cilium_vxlan",
		"ip link del vxlan.calico",
		"ip link del vxlan-v6.calico",
		"ip -br link show | grep 'cali[a-f0-9]*' | awk -F '@' '{print $1}' | xargs -r -t -n 1 ip link del",
		"ip netns show 2>/dev/null | grep cni- | xargs -r -t -n 1 ip netns del",
	}
)

type ResetNetworkConfig struct {
	common.KubeAction
}

func (r *ResetNetworkConfig) Execute(runtime connector.Runtime) error {
	for _, cmd := range networkResetCmds {
		_, _ = runtime.GetRunner().SudoCmd(cmd, true)
	}
	return nil
}

type StopKubelet struct {
	common.KubeAction
}

func (s *StopKubelet) Execute(runtime connector.Runtime) error {
	_, _ = runtime.GetRunner().SudoCmd("systemctl disable kubelet && systemctl stop kubelet && exit 0", false)
	return nil
}

type UninstallETCD struct {
	common.KubeAction
}

func (s *UninstallETCD) Execute(runtime connector.Runtime) error {
	_, _ = runtime.GetRunner().SudoCmd("systemctl stop etcd && exit 0", false)
	for _, file := range etcdFiles {
		_, _ = runtime.GetRunner().SudoCmd(fmt.Sprintf("rm -rf %s", file), true)
	}
	return nil
}

type RemoveNodeFiles struct {
	common.KubeAction
}

func (r *RemoveNodeFiles) Execute(runtime connector.Runtime) error {
	nodeFiles := []string{
		"/etc/kubernetes",
		"/etc/systemd/system/etcd.service",
		"/var/log/calico",
		"/etc/cni",
		"/var/log/pods/",
		"/var/lib/cni",
		"/var/lib/calico",
		"/var/lib/kubelet/*",
		"/run/calico",
		"/run/flannel",
		"/etc/flannel",
		"/etc/systemd/system/kubelet.service",
		"/etc/systemd/system/kubelet.service.d",
		"/usr/local/bin/kubelet",
		"/usr/local/bin/kubeadm",
		"/usr/bin/kubelet",
		"/tmp/kubekey",
		"/etc/kubekey",
	}

	for _, file := range nodeFiles {
		_, _ = runtime.GetRunner().SudoCmd(fmt.Sprintf("rm -rf %s", file), true)
	}
	return nil
}

type RemoveFiles struct {
	common.KubeAction
}

func (r *RemoveFiles) Execute(runtime connector.Runtime) error {
	for _, file := range clusterFiles {
		_, _ = runtime.GetRunner().SudoCmd(fmt.Sprintf("rm -rf %s", file), true)
	}
	// remove pki/etcd Path if it exists, otherwise it will cause the etcd reinstallation to fail if ip change
	pkiPath := fmt.Sprintf("%s/pki/etcd", runtime.GetWorkDir())
	_, _ = runtime.GetRunner().SudoCmd(fmt.Sprintf("rm -rf %s", pkiPath), true)
	return nil
}

type DaemonReload struct {
	common.KubeAction
}

func (d *DaemonReload) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("systemctl daemon-reload && exit 0", false); err != nil {
		return errors.Wrap(errors.WithStack(err), "systemctl daemon-reload failed")
	}

	// try to restart the cotainerd after /etc/cni has been removed
	_, _ = runtime.GetRunner().SudoCmd("systemctl restart containerd", false)
	return nil
}

type GetOSData struct {
	common.KubeAction
}

func (g *GetOSData) Execute(runtime connector.Runtime) error {
	osReleaseStr, err := runtime.GetRunner().SudoCmd("cat /etc/os-release", false)
	if err != nil {
		return err
	}
	osrData := osrelease.Parse(strings.Replace(osReleaseStr, "\r\n", "\n", -1))

	host := runtime.RemoteHost()
	// type: *osrelease.data
	host.GetCache().Set(Release, osrData)
	return nil
}

type GetLocalOSData struct {
	common.KubeAction
}

func (g *GetLocalOSData) Execute(runtime connector.Runtime) error {
	osReleaseStr, err := exec.Command("sh", "-c", "cat /etc/os-release").Output()
	if err != nil {
		return err
	}
	osrData := osrelease.Parse(strings.Replace(string(osReleaseStr), "\r\n", "\n", -1))

	// type: *osrelease.data
	g.ModuleCache.Set(fmt.Sprintf("local-%s", Release), osrData)

	return nil
}

type SyncRepositoryFile struct {
	common.KubeAction
}

func (s *SyncRepositoryFile) Execute(runtime connector.Runtime) error {
	if err := utils.ResetTmpDir(runtime); err != nil {
		// umount
		mountPath := filepath.Join(common.TmpDir, "iso")
		umountCmd := fmt.Sprintf("umount %s", mountPath)
		runtime.GetRunner().SudoCmd(umountCmd, false)

		// retry
		if err = utils.ResetTmpDir(runtime); err != nil {
			return errors.Wrap(err, "reset tmp dir failed")
		}
	}

	host := runtime.RemoteHost()
	release, ok := host.GetCache().Get(Release)
	if !ok {
		return errors.New("get os release failed by root cache")
	}
	r := release.(*osrelease.Data)
	isoFileName := fmt.Sprintf("%s-%s-%s.iso", r.ID, r.VersionID, host.GetArch())
	srcDir := filepath.Join(runtime.GetWorkDir(), "repository", host.GetArch(), r.ID, r.VersionID)
	files, err := os.ReadDir(srcDir)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading directory: %s", err.Error()))
	}
	for _, srcFile := range files {
		if srcFile.IsDir() {
			continue
		}
		dst := filepath.Join(common.TmpDir, srcFile.Name())
		src := filepath.Join(srcDir, srcFile.Name())
		if err := runtime.GetRunner().Scp(src, dst); err != nil {
			return errors.Wrapf(errors.WithStack(err), "scp %s to %s failed", src, dst)
		}
		if strings.Compare(isoFileName, srcFile.Name()) == 0 {
			host.GetCache().Set("iso", srcFile.Name())
		} else {
			host.GetCache().Set(fmt.Sprintf("file-%s", srcFile.Name()), srcFile.Name())
		}
	}
	return nil
}

type SyncRepositoryLocalFiles struct {
	common.KubeAction
}

func (s *SyncRepositoryLocalFiles) Execute(runtime connector.Runtime) error {
	if err := utils.ResetLocalTmpDir(); err != nil {
		// umount
		mountPath := filepath.Join(common.TmpDir, "iso")
		umountCmd := fmt.Sprintf("umount %s", mountPath)
		err = exec.Command("sh", "-c", umountCmd).Run()

		// retry
		if err = utils.ResetLocalTmpDir(); err != nil {
			return errors.Wrap(err, "reset tmp dir failed")
		}
	}

	host := runtime.RemoteHost()
	release, ok := s.ModuleCache.Get(fmt.Sprintf("local-%s", Release))
	if !ok {
		return errors.New("get os release failed by root cache")
	}
	r := release.(*osrelease.Data)
	isoFileName := fmt.Sprintf("%s-%s-%s.iso", r.ID, r.VersionID, host.GetArch())
	srcDir := filepath.Join(runtime.GetWorkDir(), "repository", host.GetArch(), r.ID, r.VersionID)
	files, err := os.ReadDir(srcDir)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading directory: %s", err.Error()))
	}
	for _, srcFile := range files {
		if srcFile.IsDir() {
			continue
		}
		dst := filepath.Join(common.TmpDir, srcFile.Name())
		src := filepath.Join(srcDir, srcFile.Name())

		err = CopyFile(src, dst)
		if err != nil {
			return errors.Wrapf(errors.WithStack(err), "cp %s to %s failed", src, dst)
		}
		if strings.Compare(isoFileName, srcFile.Name()) == 0 {
			s.ModuleCache.Set("iso", srcFile.Name())
		} else {
			s.ModuleCache.Set(fmt.Sprintf("file-%s", srcFile.Name()), srcFile.Name())
		}
	}
	return nil
}

type MountISO struct {
	common.KubeAction
}

func (m *MountISO) Execute(runtime connector.Runtime) error {
	mountPath := filepath.Join(common.TmpDir, "iso")
	if err := runtime.GetRunner().MkDir(mountPath); err != nil {
		return errors.Wrapf(errors.WithStack(err), "create mount dir failed")
	}

	host := runtime.RemoteHost()
	isoFile, _ := host.GetCache().GetMustString("iso")
	path := filepath.Join(common.TmpDir, isoFile)
	mountCmd := fmt.Sprintf("sudo mount -t iso9660 -o loop %s %s", path, mountPath)
	if _, err := runtime.GetRunner().Cmd(mountCmd, false); err != nil {
		return errors.Wrapf(errors.WithStack(err), "mount %s at %s failed", path, mountPath)
	}
	return nil
}

type LocalMountISO struct {
	common.KubeAction
}

func (m *LocalMountISO) Execute(runtime connector.Runtime) error {
	mountPath := filepath.Join(common.TmpDir, "iso")

	if err := exec.Command("sh", "-c", "mkdir -p "+mountPath).Run(); err != nil {
		return errors.Wrapf(errors.WithStack(err), "create mount dir failed")
	}

	isoFile, _ := m.ModuleCache.GetMustString("iso")
	path := filepath.Join(common.TmpDir, isoFile)
	mountCmd := fmt.Sprintf("sudo mount -t iso9660 -o loop %s %s", path, mountPath)

	if err := exec.Command("sh", "-c", mountCmd).Run(); err != nil {
		return errors.Wrapf(errors.WithStack(err), "mount %s at %s failed", path, mountPath)
	}
	return nil
}

type NewRepoServer struct {
	common.KubeAction
}

func (n *NewRepoServer) Execute(runtime connector.Runtime) error {
	//设置要提供的文件目录
	go func() {
		dir := filepath.Join(common.TmpDir, "iso")

		// 创建文件服务器
		fs := http.FileServer(http.Dir(dir))

		// 将文件服务器的处理程序注册到根路径
		http.Handle("/", fs)

		// 设置服务器端口
		port := ":8888"
		log.Printf("server starting for port to %s ，run dir: %s", port, dir)

		// 启动 HTTP 服务器
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatalf("server start fail: %v", err)
		}

	}()

	return nil
}

type NewRepoClient struct {
	common.KubeAction
}

func (n *NewRepoClient) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	release, ok := host.GetCache().Get(Release)
	if !ok {
		return errors.New("get os release failed by host cache")
	}
	r := release.(*osrelease.Data)

	repo, err := repository.New(r.ID)
	if err != nil {
		checkDeb, debErr := runtime.GetRunner().SudoCmd("which apt", false)
		if debErr == nil && strings.Contains(checkDeb, "bin") {
			repo = repository.NewDeb()
		}
		checkRPM, rpmErr := runtime.GetRunner().SudoCmd("which yum", false)
		if rpmErr == nil && strings.Contains(checkRPM, "bin") {
			repo = repository.NewRPM()
		}

		if debErr != nil && rpmErr != nil {
			return errors.Wrap(errors.WithStack(err), "new repository manager failed")
		} else if debErr == nil && rpmErr == nil {
			return errors.New("can't detect the main package repository, only one of apt or yum is supported")
		}
	}

	host.GetCache().Set("repo", repo)
	return nil
}

type BackupOriginalRepository struct {
	common.KubeAction
}

func (b *BackupOriginalRepository) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	r, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	repo := r.(repository.Interface)

	if err := repo.Backup(runtime); err != nil {
		return errors.Wrap(errors.WithStack(err), "backup repository failed")
	}

	return nil
}

type AddLocalRepository struct {
	common.KubeAction
}

func (a *AddLocalRepository) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	r, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	repo := r.(repository.Interface)
	serverIp := util.LocalIP()
	if a.KubeConf.Arg.RepositoryIp != nil {
		serverIp = a.KubeConf.Arg.RepositoryIp.String()
	}
	if installErr := repo.Add(runtime, fmt.Sprintf("http://%s:8888", serverIp)); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "add local repository failed")
	}
	if installErr := repo.Update(runtime); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "update local repository failed")
	}

	return nil
}

type InstallPackage struct {
	common.KubeAction
}

func (i *InstallPackage) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	repo, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	r := repo.(repository.Interface)

	var pkg []string
	if _, ok := r.(*repository.Debian); ok {
		pkg = i.KubeConf.Cluster.System.Debs
	} else if _, ok := r.(*repository.RedhatPackageManager); ok {
		pkg = i.KubeConf.Cluster.System.Rpms
	}

	if installErr := r.Update(runtime); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "update repository failed")
	}

	if installErr := r.Install(runtime, pkg...); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "install repository package failed")
	}
	return nil
}

type PreInstallPackage struct {
	common.KubeAction
}

func (i *PreInstallPackage) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	repo, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	r := repo.(repository.Interface)

	var pkg []string
	if _, ok := r.(*repository.Debian); ok {
		pkg = i.KubeConf.Cluster.System.PreDebs
	} else if _, ok := r.(*repository.RedhatPackageManager); ok {
		pkg = i.KubeConf.Cluster.System.PreRpms
	}

	if installErr := r.Update(runtime); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "update repository failed")
	}

	if installErr := r.Install(runtime, pkg...); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "install repository package failed")
	}
	return nil
}

type PostInstallPackage struct {
	common.KubeAction
}

func (i *PostInstallPackage) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	repo, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	r := repo.(repository.Interface)

	var pkg []string
	if _, ok := r.(*repository.Debian); ok {
		pkg = i.KubeConf.Cluster.System.PostDebs
	} else if _, ok := r.(*repository.RedhatPackageManager); ok {
		pkg = i.KubeConf.Cluster.System.PostRpms
	}

	if installErr := r.Update(runtime); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "update repository failed")
	}

	if installErr := r.Install(runtime, pkg...); installErr != nil {
		return errors.Wrap(errors.WithStack(installErr), "install repository package failed")
	}
	return nil
}

type CustomAfterPreInstall struct {
	common.KubeAction
}

func (i *CustomAfterPreInstall) Execute(runtime connector.Runtime) error {
	for idx, scriptsItem := range i.KubeConf.Cluster.System.AfterPrePkgs {
		taskDir := fmt.Sprintf("%s-%d-script", "CustomAfterPreInstall", idx)
		if len(scriptsItem.Bash) <= 0 {
			return errors.Errorf("custom script %s Bash is empty", scriptsItem.Name)
		}
		err := RunCustomScripts(runtime, scriptsItem, taskDir)
		if err != nil {
			return err
		}
	}
	return nil
}

type CustomAfterPostInstall struct {
	common.KubeAction
}

func (i *CustomAfterPostInstall) Execute(runtime connector.Runtime) error {
	for idx, scriptsItem := range i.KubeConf.Cluster.System.AfterPostPkgs {
		taskDir := fmt.Sprintf("%s-%d-script", "CustomAfterPreInstall", idx)
		if len(scriptsItem.Bash) <= 0 {
			return errors.Errorf("custom script %s Bash is empty", scriptsItem.Name)
		}
		err := RunCustomScripts(runtime, scriptsItem, taskDir)
		if err != nil {
			return err
		}
	}
	return nil
}

type ResetRepository struct {
	common.KubeAction
}

func (r *ResetRepository) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()
	repo, ok := host.GetCache().Get("repo")
	if !ok {
		return errors.New("get repo failed by host cache")
	}
	re := repo.(repository.Interface)

	var resetErr error
	defer func() {
		if resetErr != nil {
			mountPath := filepath.Join(common.TmpDir, "iso")
			umountCmd := fmt.Sprintf("umount %s", mountPath)
			_, _ = runtime.GetRunner().SudoCmd(umountCmd, false)
		}
	}()

	if resetErr = re.Reset(runtime); resetErr != nil {
		return errors.Wrap(errors.WithStack(resetErr), "reset repository failed")
	}

	return nil
}

type UmountISO struct {
	common.KubeAction
}

func (u *UmountISO) Execute(runtime connector.Runtime) error {
	mountPath := filepath.Join(common.TmpDir, "iso")
	umountCmd := fmt.Sprintf("umount %s", mountPath)
	if _, err := runtime.GetRunner().SudoCmd(umountCmd, false); err != nil {
		return errors.Wrapf(errors.WithStack(err), "umount %s failed", mountPath)
	}
	return nil
}

type LocalUmountISO struct {
	common.KubeAction
}

func (u *LocalUmountISO) Execute(runtime connector.Runtime) error {
	mountPath := filepath.Join(common.TmpDir, "iso")
	umountCmd := fmt.Sprintf("umount %s", mountPath)
	if err := exec.Command("sh", "-c", umountCmd).Run(); err != nil {
		return errors.Wrapf(errors.WithStack(err), "umount %s failed", mountPath)
	}
	return nil
}

type NodeConfigureNtpServer struct {
	common.KubeAction
}

func (n *NodeConfigureNtpServer) Execute(runtime connector.Runtime) error {

	currentHost := runtime.RemoteHost()
	release, ok := currentHost.GetCache().Get(Release)
	if !ok {
		return errors.New("get os release failed by host cache")
	}
	r := release.(*osrelease.Data)

	chronyConfigFile := "/etc/chrony.conf"
	chronyService := "chronyd.service"
	if r.ID == "ubuntu" || r.ID == "debian" {
		chronyConfigFile = "/etc/chrony/chrony.conf"
		chronyService = "chrony.service"
	}

	clearOldServerCmd := fmt.Sprintf(`sed -i '/^server/d' %s`, chronyConfigFile)
	if _, err := runtime.GetRunner().SudoCmd(clearOldServerCmd, false); err != nil {
		return errors.Wrapf(err, "delete old servers failed, please check file %s", chronyConfigFile)
	}

	poolDisableCmd := fmt.Sprintf(`sed -i 's/^pool /#pool /g' %s`, chronyConfigFile)
	if _, err := runtime.GetRunner().SudoCmd(poolDisableCmd, false); err != nil {
		return errors.Wrapf(err, "set pool disable failed")
	}
	// if NtpServers was configured
	for _, server := range n.KubeConf.Cluster.System.NtpServers {

		serverAddr := strings.Trim(server, " \"")
		fmt.Printf("ntpserver: %s, current host: %s\n", serverAddr, currentHost.GetName())
		if serverAddr == currentHost.GetName() || serverAddr == currentHost.GetInternalIPv4Address() {
			deleteAllowCmd := fmt.Sprintf(`sed -i '/^allow/d' %s`, chronyConfigFile)
			if _, err := runtime.GetRunner().SudoCmd(deleteAllowCmd, false); err != nil {
				return errors.Wrapf(err, "delete allow failed, please check file %s", chronyConfigFile)
			}
			allowClientCmd := fmt.Sprintf(`echo 'allow 0.0.0.0/0' >> %s`, chronyConfigFile)
			if _, err := runtime.GetRunner().SudoCmd(allowClientCmd, false); err != nil {
				return errors.Wrapf(err, "change host:%s chronyd conf failed, please check file %s", serverAddr, chronyConfigFile)
			}
			deleteLocalCmd := fmt.Sprintf(`sed -i '/^local/d' %s`, chronyConfigFile)
			if _, err := runtime.GetRunner().SudoCmd(deleteLocalCmd, false); err != nil {
				return errors.Wrapf(err, "delete local stratum failed, please check file %s", chronyConfigFile)
			}
			AddLocalCmd := fmt.Sprintf(`echo 'local stratum 10' >> %s`, chronyConfigFile)
			if _, err := runtime.GetRunner().SudoCmd(AddLocalCmd, false); err != nil {
				return errors.Wrapf(err, "Add local stratum 10 conf failed, please check file %s", chronyConfigFile)
			}
		}

		// use internal ip to client chronyd server
		for _, host := range runtime.GetAllHosts() {
			if serverAddr == host.GetName() {
				serverAddr = host.GetInternalIPv4Address()
				break
			}
		}

		checkOrAddCmd := fmt.Sprintf(`grep -q '^server %s iburst' %s||sed '1a server %s iburst' -i %s`, serverAddr, chronyConfigFile, serverAddr, chronyConfigFile)
		if _, err := runtime.GetRunner().SudoCmd(checkOrAddCmd, false); err != nil {
			return errors.Wrapf(err, "set ntpserver: %s failed, please check file %s", serverAddr, chronyConfigFile)
		}

	}

	// if Timezone was configured
	if len(n.KubeConf.Cluster.System.Timezone) > 0 {
		setTimeZoneCmd := fmt.Sprintf("timedatectl set-timezone %s", n.KubeConf.Cluster.System.Timezone)
		if _, err := runtime.GetRunner().SudoCmd(setTimeZoneCmd, false); err != nil {
			return errors.Wrapf(err, "set timezone: %s failed", n.KubeConf.Cluster.System.Timezone)
		}

		if _, err := runtime.GetRunner().SudoCmd("timedatectl set-ntp true", false); err != nil {
			return errors.Wrap(err, "timedatectl set-ntp true failed")
		}
	}

	// ensure chronyd was enabled and work normally
	if len(n.KubeConf.Cluster.System.NtpServers) > 0 || len(n.KubeConf.Cluster.System.Timezone) > 0 {
		startChronyCmd := fmt.Sprintf("systemctl enable %s && systemctl restart %s", chronyService, chronyService)
		if _, err := runtime.GetRunner().SudoCmd(startChronyCmd, false); err != nil {
			return errors.Wrap(err, "restart chronyd failed")
		}

		// tells chronyd to cancel any remaining correction that was being slewed and jump the system clock by the equivalent amount, making it correct immediately.
		if _, err := runtime.GetRunner().SudoCmd("chronyc makestep > /dev/null && chronyc sources", true); err != nil {
			return errors.Wrap(err, "chronyc makestep failed")
		}
	}

	return nil
}

type ConfigAicpRootDir struct {
	common.KubeAction
}

func (g *ConfigAicpRootDir) Execute(runtime connector.Runtime) error {
	// Get current host information
	currentHost := getCurrentHost(runtime)
	if currentHost == nil {
		return errors.New("current host not found")
	}

	if currentHost.DockerRootDisk == "" {
		return nil
	}

	// Create directory
	mkdirCmd := fmt.Sprintf("mkdir -p %s", common.AicpDockerRootDir)
	_, err := runtime.GetRunner().SudoCmd(mkdirCmd, false)
	if err != nil {
		return errors.Wrap(err, "failed to create aicp directory")
	}

	if !checkFileSystemFormattedAsXfs(runtime, currentHost.DockerRootDisk) {
		// Format file system
		formatCmd := fmt.Sprintf("mkfs.xfs -f %s", currentHost.DockerRootDisk)
		_, err := runtime.GetRunner().SudoCmd(formatCmd, false)
		if err != nil {
			return errors.Wrap(err, "failed to format disk with xfs")
		}
	}

	if !checkMountExists(runtime, common.AicpDockerRootDir) {
		// Mount file system
		mountCmd := fmt.Sprintf("mount -o prjquota %s %s", currentHost.DockerRootDisk, common.AicpDockerRootDir)
		_, err = runtime.GetRunner().SudoCmd(mountCmd, false)
		if err != nil {
			return errors.Wrap(err, "failed to mount prjquota")
		}
	}

	if !checkFstabEntryExists(runtime, common.AicpDockerRootDir) {
		// Get UUID of file system
		uuid, err := getDeviceUUID(runtime, currentHost.DockerRootDisk)
		if err != nil || uuid == "" {
			return errors.Wrap(err, "failed to get UUID")
		}

		// Add mount information to /etc/fstab
		fstabCmd := fmt.Sprintf("echo 'UUID=%s %s xfs defaults,prjquota 0 1' >> /etc/fstab", uuid, common.AicpDockerRootDir)
		_, err = runtime.GetRunner().SudoCmd(fstabCmd, false)
		if err != nil {
			return errors.Wrap(err, "failed to add fstab entry")
		}
	}

	return nil
}

type ConfigAicpDataDir struct {
	common.KubeAction
}

func (g *ConfigAicpDataDir) Execute(runtime connector.Runtime) error {
	// Get current host information
	currentHost := getCurrentHost(runtime)
	if currentHost == nil {
		return errors.New("current host not found")
	}

	if currentHost.ZfsDataDisk == nil || len(currentHost.ZfsDataDisk) == 0 {
		return nil
	}

	modprobeCmd := "modprobe zfs"
	runtime.GetRunner().SudoCmd(modprobeCmd, false)

	if !isZfsPoolCreated(runtime, common.AicpZpoolName) {
		zfsCmd := fmt.Sprintf("zpool create %s %s", common.AicpZpoolName, strings.Join(currentHost.ZfsDataDisk, " "))
		_, err := runtime.GetRunner().SudoCmd(zfsCmd, false)
		if err != nil {
			return errors.Wrap(err, "failed to create zpool")
		}
	}

	return nil
}

func isZfsPoolCreated(runtime connector.Runtime, aicpZpoolName string) bool {
	cmd := "zpool list"
	out, err := runtime.GetRunner().SudoCmd(cmd, false)
	if err != nil {
		// Handle error
		return false
	}

	return strings.Contains(out, aicpZpoolName)
}

// Get current host information
func getCurrentHost(runtime connector.Runtime) *kubekeyv1alpha2.KubeHost {
	currentHost := runtime.RemoteHost().GetName()
	hosts := runtime.GetHostsByRole(common.K8s)
	for _, host := range hosts {
		kubeHost := host.(*kubekeyv1alpha2.KubeHost)
		if kubeHost.Name == currentHost {
			return kubeHost
		}
	}
	return nil
}

// Check if device is formatted as xfs
func checkFileSystemFormattedAsXfs(runtime connector.Runtime, device string) bool {
	formatCmd := fmt.Sprintf("blkid -s TYPE -o value %s", device)
	out, err := runtime.GetRunner().SudoCmd(formatCmd, false)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == "xfs"
}

// Check if file dir is mounted
func checkMountExists(runtime connector.Runtime, dir string) bool {
	mountCmd := fmt.Sprintf("mountpoint -q %s", dir)
	_, err := runtime.GetRunner().SudoCmd(mountCmd, false)
	if err != nil {
		return false
	}
	return true
}

// Check if entry is added to /etc/fstab
func checkFstabEntryExists(runtime connector.Runtime, device string) bool {
	fstabCmd := "cat /etc/fstab"
	fstabContent, err := runtime.GetRunner().SudoCmd(fstabCmd, false)
	if err != nil {
		// Handle error
		return false
	}
	return strings.Contains(fstabContent, device)
}

// Get UUID of device
func getDeviceUUID(runtime connector.Runtime, device string) (string, error) {
	getUuidCmd := fmt.Sprintf("blkid -s UUID -o value %s", device)
	uuid, err := runtime.GetRunner().SudoCmd(getUuidCmd, false)
	if err != nil || uuid == "" {
		return "", err
	}
	return strings.TrimSpace(uuid), nil
}

// Run custom scripts
func RunCustomScripts(runtime connector.Runtime, scripts kubekeyv1alpha2.CustomScripts, taskDir string) error {
	remoteTaskHome := common.TmpDir + taskDir

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("mkdir -p %s", remoteTaskHome), false); err != nil {
		return errors.Wrapf(err, "create remoteTaskHome: %s  err:%s", remoteTaskHome, err)
	}

	// dilver the dependency materials to the remotehost
	for idx, localPath := range scripts.Materials {

		if !util.IsExist(localPath) {
			return errors.Errorf("Not found Path: %s", localPath)
		}

		targetPath := filepath.Join(remoteTaskHome, filepath.Base(localPath))

		// first clean the target to makesure target path always is the lastest.
		cleanCmd := fmt.Sprintf("rm -fr %s", targetPath)
		if _, err := runtime.GetRunner().SudoCmd(cleanCmd, false); err != nil {
			return errors.Wrapf(err, "Can not remove target found Path: %s", targetPath)
		}

		start := time.Now()
		err := runtime.GetRunner().SudoScp(localPath, targetPath)
		if err != nil {
			return errors.Wrapf(err, "Can not Scp -fr %s root@%s:%s", localPath, runtime.RemoteHost().GetAddress(), targetPath)
		}

		fmt.Printf("Copy %d/%d materials: Scp -fr %s root@%s:%s done, take %s\n",
			idx, len(scripts.Materials), localPath, runtime.RemoteHost().GetAddress(), targetPath, time.Since(start))
	}

	// wrap use bash file if shell has many lines.
	RunBash := scripts.Bash
	if strings.Index(RunBash, "\n") > 0 {
		tmpFile, err := os.CreateTemp(os.TempDir(), taskDir)
		if err != nil {
			return errors.Wrapf(err, "create tmp Bash: %s/%s in local node, err:%s", os.TempDir(), taskDir, err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(RunBash); err != nil {
			return errors.Wrapf(err, "write to tmp:%s in local node, err:%s", tmpFile.Name(), err)
		}

		targetPath := filepath.Join(remoteTaskHome, "task.sh")
		if err := runtime.GetRunner().SudoScpWithoutDelete(tmpFile.Name(), targetPath); err != nil {
			return errors.Wrapf(err, "Can not Scp -fr %s root@%s:%s", tmpFile.Name(), runtime.RemoteHost().GetAddress(), targetPath)
		}

		RunBash = "/bin/bash " + targetPath
	}

	start := time.Now()
	out, err := runtime.GetRunner().SudoCmd(RunBash, false)
	if err != nil {
		return errors.Errorf("Exec Bash: %s err:%s", RunBash, err)
	}

	if !runtime.GetRunner().Debug {
		fmt.Printf("Exec Bash:%s done, take %s", RunBash, time.Since(start))
		cleanCmd := fmt.Sprintf("rm -fr %s", remoteTaskHome)
		if _, err := runtime.GetRunner().SudoCmd(cleanCmd, false); err != nil {
			return errors.Wrapf(err, "Exec cmd:%s err:%s", cleanCmd, err)
		}
	} else {
		// keep the Materials for debug
		fmt.Printf("Exec Bash:%s done, take %s, output:\n%s", RunBash, time.Since(start), out)
	}
	return nil
}
