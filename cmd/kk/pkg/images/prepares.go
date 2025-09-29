package images

import (
	"strings"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
)

type HarborExist struct {
	common.KubePrepare
	Not bool
}

func (r *HarborExist) PreCheck(runtime connector.Runtime) (bool, error) {

	hosts := runtime.GetHostsByRole(common.Registry)
	if len(hosts) == 0 {
		return false, nil
	}
	var conn connector.Connection
	var err error

	if runtime.GetRunner().Conn == nil {
		conn, err = runtime.GetConnector().Connect(hosts[0])
		if err != nil {
			return false, err
		}
	} else {
		conn = runtime.GetRunner().Conn
	}

	switch r.KubeConf.Cluster.Registry.Type {
	case common.Harbor:
		output, _, err := conn.Exec(connector.SudoPrefix("systemctl is-active harbor"), hosts[0])
		if err != nil {
			return r.Not, nil
		}
		if strings.Contains(output, "active") {
			return !r.Not, nil
		}
		return r.Not, nil
	case common.Registry:
		return false, nil
	}

	return false, nil
}
