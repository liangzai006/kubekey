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

package storage

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/plugins/aicp"
)

type DeployLocalVolume struct {
	common.KubeAction
}

func (d *DeployLocalVolume) Execute(runtime connector.Runtime) error {
	cmd := fmt.Sprintf("/usr/local/bin/kubectl apply -f %s", filepath.Join(common.KubeAddonsDir, "local-volume.yaml"))
	if _, err := runtime.GetRunner().SudoCmd(cmd, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "deploy local-volume.yaml failed")
	}
	return nil
}

type DeployZfsStorageClass struct {
	common.KubeAction
}

func (d *DeployZfsStorageClass) Execute(runtime connector.Runtime) error {
	zfsDir := filepath.Join(d.KubeConf.Arg.AicpWorkDir, "charts", "zfs-localpv")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}

	helm := aicp.HelmOptions{
		Name:      "zfs-localpv",
		Namespace: "openebs-system",
		ChartPath: zfsDir,
		Values:    vals,
	}
	return helm.Install()
}

type DeployLongHornStorageClass struct {
	common.KubeAction
}

func (d *DeployLongHornStorageClass) Execute(runtime connector.Runtime) error {
	longHornDir := filepath.Join(d.KubeConf.Arg.AicpWorkDir, "charts", "longhorn")
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"imageRegistry": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}

	helm := aicp.HelmOptions{
		Name:      "longhorn",
		Namespace: "longhorn-system",
		ChartPath: longHornDir,
		Values:    vals,
	}
	return helm.Install()
}

type AicpStorageTask struct {
	common.KubeAction
}

func (a *AicpStorageTask) Execute(runtime connector.Runtime) error {
	aicpStorageDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "charts", "aicp-storage")

	iaasKeys, ok := a.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"users": map[string]interface{}{
			"aicpName":       common.PG_AICP,
			"aicpPassword":   iaasKeys.(map[string]string)[common.PG_AICP],
			"yunifyName":     common.PG_YUNIFY,
			"yunifyPassword": iaasKeys.(map[string]string)[common.PG_YUNIFY],
		},
		"redis": map[string]interface{}{
			"password": iaasKeys.(map[string]string)[common.REDIS_PASSWORD],
		},
	}
	helm := aicp.HelmOptions{
		Name:      "aicp-storage",
		Namespace: "aicp-storage",
		ChartPath: aicpStorageDir,
		Values:    vals,
	}
	return helm.Install()
}
