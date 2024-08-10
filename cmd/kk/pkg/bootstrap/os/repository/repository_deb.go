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

package repository

import (
	"fmt"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"strings"
	"time"
)

type Debian struct {
	backup bool
}

func NewDeb() Interface {
	return &Debian{}
}

func (d *Debian) Backup(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("if [ -f /etc/apt/sources.list ]; then mv /etc/apt/sources.list /etc/apt/sources.list.kubekey.bak; fi", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd("rm -rf /etc/apt/sources.list.d.kubekey.bak", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd("if [ -d /etc/apt/sources.list.d ]; then mv /etc/apt/sources.list.d /etc/apt/sources.list.d.kubekey.bak; fi", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd("mkdir -p /etc/apt/sources.list.d", false); err != nil {
		return err
	}
	d.backup = true
	return nil
}

func (d *Debian) IsAlreadyBackUp() bool {
	return d.backup
}

func (d *Debian) Add(runtime connector.Runtime, path string) error {
	if !d.IsAlreadyBackUp() {
		return fmt.Errorf("linux repository must be backuped before")
	}

	if _, err := runtime.GetRunner().SudoCmd("rm -rf /etc/apt/sources.list.d/*", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("echo 'deb [trusted=yes]  file://%s   /' > /etc/apt/sources.list.d/kubekey.list", path),
		true); err != nil {
		return err
	}
	return nil
}

func (d *Debian) Update(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().Cmd("sudo apt-get update", true); err != nil {
		return err
	}
	return nil
}

func (d *Debian) Install(runtime connector.Runtime, pkg ...string) error {
	defaultPkg := []string{"socat", "conntrack", "ipset", "ebtables", "chrony", "ipvsadm"}
	if len(pkg) == 0 {
		pkg = defaultPkg
	} else {
		pkg = append(pkg, defaultPkg...)
	}

	str := strings.Join(pkg, " ")
	installCmd := fmt.Sprintf("timeout 300 apt install -y %s", str)
	maxRetries := 5
	for attempts := 1; attempts <= maxRetries; attempts++ {
		if _, err := runtime.GetRunner().SudoCmd(installCmd, true); err != nil {
			// If an error occurs and we have not reached the max retries, wait and retry
			if attempts < maxRetries {
				killCmd := "ps aux | grep apt | grep install | grep -v grep | awk '{print $2}' | xargs -r kill -9"
				runtime.GetRunner().SudoCmd(killCmd, true)
				repairCmd := "dpkg --configure -a"
				runtime.GetRunner().SudoCmd(repairCmd, true)
				time.Sleep(5 * time.Second)
				fixCmd := "echo y | apt --fix-broken install"
				runtime.GetRunner().SudoCmd(fixCmd, true)
				continue
			}
			// If we have reached the max retries, return the last error
			return err
		}
		// If the command was successful, break out of the loop
		break
	}
	return nil
}

func (d *Debian) Reset(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("rm -rf /etc/apt/sources.list.d", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd("if [ -f /etc/apt/sources.list.kubekey.bak ]; then mv /etc/apt/sources.list.kubekey.bak /etc/apt/sources.list; fi", false); err != nil {
		return err
	}

	if _, err := runtime.GetRunner().SudoCmd("if [ -d /etc/apt/sources.list.d.kubekey.bak ]; then mv /etc/apt/sources.list.d.kubekey.bak /etc/apt/sources.list.d; fi", false); err != nil {
		return err
	}

	return nil
}
