package aicp

import (
	"database/sql"
	"fmt"
	"regexp"

	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog/v2"
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

type PatchNginxTask struct {
	common.KubeAction
}

func (p *PatchNginxTask) Execute(runtime connector.Runtime) error {
	nginxDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "nginx")

	helm := HelmOptions{
		Name:      "nginx",
		Namespace: "pitrix",
		ChartPath: nginxDir,
	}

	rels, err := helm.GetHistoryRelease()
	if err != nil {
		return err
	}
	var lastRel *release.Release
	for _, rel := range rels {
		if rel.Info.Status == release.StatusDeployed {
			lastRel = rel
			break
		}
	}
	if lastRel == nil {
		klog.Warningf("no deployed release found, %w", driver.ErrReleaseNotFound)
		lastRel = rels[0]
	}
	// current nginx version is less than 2.27.0, please upgrade nginx to 2.27.0 or later
	if versionutil.MustParseSemantic(lastRel.Chart.Metadata.Version).LessThan(versionutil.MustParseSemantic("2.27.0")) {
		return fmt.Errorf("current nginx version %s is less than 2.27.0, please upgrade nginx to 2.27.0 or later", lastRel.Chart.Metadata.Version)
	}

	// 备份manifest
	backupPath := filepath.Join(runtime.GetWorkDir(), "backup")

	var backup []string

	backup = append(backup, lastRel.Manifest)
	for _, hook := range lastRel.Hooks {
		backup = append(backup, hook.Manifest)
	}
	data, err := yaml.Marshal(lastRel.Config)
	if err != nil {
		return err
	}
	backup = append(backup, string(data))

	str := strings.Join(backup, "\n---\n")
	err = os.WriteFile(filepath.Join(backupPath, fmt.Sprintf("backup-%d.yaml", lastRel.Version)), []byte(str), 0644)
	if err != nil {
		return err
	}

	regValues := lastRel.Config

	valuesMap := regValues["configMap"].(map[string]interface{})
	zones := valuesMap["zones"].([]map[string]interface{})
	valuesMap["zones"] = append(zones, map[string]interface{}{
		"name":     p.KubeConf.Cluster.Aicp.Zone,
		"endpoint": fmt.Sprintf("%s:31080", p.KubeConf.Cluster.ControlPlaneEndpoint.Address),
	})

	helm.Values = regValues

	return helm.Upgrade()
}

type PatchZoneDbTask struct {
	common.KubeAction
}

func (p *PatchZoneDbTask) Execute(runtime connector.Runtime) error {

	dbUrl := fmt.Sprintf("host=localhost port=5432 user=%s password=%s sslmode=disable", "yunify", "zhu88jie")

	klog.Infof("正在建立 port-forward 连接...")
	stopChan, readyChan, errChan := make(chan struct{}, 1), make(chan struct{}, 1), make(chan error, 1)
	defer close(stopChan)

	go func() {
		err := PortForwardToService(p.KubeConf.Arg.KubeConfig, "aicp-storage", "pg-readwrite", 5432, 5432, stopChan, readyChan)
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case <-readyChan:
		klog.Infof("Port-forward 已建立")
	case err := <-errChan:
		return fmt.Errorf("Port-forward 失败: %w", err)
	case <-time.After(10 * time.Second):
		return fmt.Errorf("Port-forward 超时")
	}

	accountDb, err := sql.Open("postgres", fmt.Sprintf("%s dbname=account", dbUrl))
	if err != nil {
		return fmt.Errorf("连接account数据库失败: %w", err)
	}

	var zones string
	err = accountDb.QueryRow("select c.zones from  console c  where c.console_id ='qacloud'").Scan(&zones)
	if err != nil {
		return fmt.Errorf("查询account zones失败: %w", err)
	}

	coreZone := regexp.MustCompile(fmt.Sprintf("\b%s\b", p.KubeConf.Cluster.Aicp.Zone))
	if !coreZone.MatchString(zones) {
		err = execSql(accountDb, fmt.Sprintf("UPDATE console SET zones = zones ||',%s' where console_id = 'qacloud'", p.KubeConf.Cluster.Aicp.Zone))
		if err != nil {
			return fmt.Errorf("更新account zones失败: %w", err)
		}
	}
	accountDb.Close()
	globalDb, err := sql.Open("postgres", fmt.Sprintf("%s dbname=global", dbUrl))
	if err != nil {
		return fmt.Errorf("连接global数据库失败: %w", err)
	}

	var regionCount int
	err = globalDb.QueryRow("select count(*) from region where region_id = ?", p.KubeConf.Cluster.Aicp.Zone).Scan(&regionCount)
	if err != nil {
		return fmt.Errorf("查询global region失败: %w", err)
	}
	if regionCount == 0 {
		err = execSql(globalDb, fmt.Sprintf("INSERT INTO region (region_id, region_name) VALUES ('%s', '%s')", p.KubeConf.Cluster.Aicp.Zone, strings.ToUpper(p.KubeConf.Cluster.Aicp.Zone)))
		if err != nil {
			return fmt.Errorf("插入global region失败: %w, sql: %s", err, fmt.Sprintf("INSERT INTO region (region_id, region_name) VALUES ('%s', '%s')", p.KubeConf.Cluster.Aicp.Zone, strings.ToLower(p.KubeConf.Cluster.Aicp.Zone)))
		}
	}

	var zoneCount int
	err = globalDb.QueryRow("select count(*) from zone where zone_id = ?", p.KubeConf.Cluster.Aicp.Zone).Scan(&zoneCount)
	if err != nil {
		return fmt.Errorf("查询global zone失败: %w, sql: %s", err, fmt.Sprintf("select count(*) from zone where zone_id = %s", p.KubeConf.Cluster.Aicp.Zone))
	}
	if zoneCount == 0 {
		var allZoneCount int
		err = globalDb.QueryRow("select count(*) from zone").Scan(&allZoneCount)
		if err != nil {
			return fmt.Errorf("查询所有global zone失败: %w, sql: %s", err, "select count(*) from zone")
		}
		sql := fmt.Sprintf("INSERT INTO zone (zone_id, zone_code,zone_name, status, front_gates, region_id, visibility) VALUES ('%s', '%d', '%s', 'active', 'global-proxy:6545', '', 'public')", p.KubeConf.Cluster.Aicp.Zone, allZoneCount+1, fmt.Sprintf("智算%d区", allZoneCount+1))
		err = execSql(globalDb, sql)
		if err != nil {
			return fmt.Errorf("插入global zone失败: %w, sql: %s", err, sql)
		}
	}

	var userZoneAdminCount int
	err = globalDb.QueryRow("select count(*) from user_zone where user_id = 'admin' and zone_id = ? and role = 'global_admin'", p.KubeConf.Cluster.Aicp.Zone).Scan(&userZoneAdminCount)
	if err != nil {
		return fmt.Errorf("查询global user_zone失败: %w, sql: %s", err, fmt.Sprintf("select count(*) from user_zone where user_id = 'admin' and zone_id = %s and role = 'global_admin'", p.KubeConf.Cluster.Aicp.Zone))
	}
	if userZoneAdminCount == 0 {
		sql := fmt.Sprintf("INSERT INTO user_zone (user_id, zone_id, privilege, role) VALUES ('admin', '%s', 10, 'global_admin')", p.KubeConf.Cluster.Aicp.Zone)
		err = execSql(globalDb, sql)
		if err != nil {
			return fmt.Errorf("插入global user_zone失败: %w, sql: %s", err, sql)
		}
	}

	var userZoneBossCount int
	err = globalDb.QueryRow("select count(*) from user_zone where user_id = 'boss' and zone_id = ? and role = 'global_admin'", p.KubeConf.Cluster.Aicp.Zone).Scan(&userZoneBossCount)
	if err != nil {
		return fmt.Errorf("查询global user_zone失败: %w, sql: %s", err, fmt.Sprintf("select count(*) from user_zone where user_id = 'boss' and zone_id = %s and role = 'global_admin'", p.KubeConf.Cluster.Aicp.Zone))
	}
	if userZoneBossCount == 0 {
		sql := fmt.Sprintf("INSERT INTO user_zone (user_id, zone_id, privilege, role) VALUES ('boss', '%s', 10, 'global_admin')", p.KubeConf.Cluster.Aicp.Zone)
		err = execSql(globalDb, sql)
		if err != nil {
			return fmt.Errorf("插入global user_zone失败: %w, sql: %s", err, sql)
		}
	}
	globalDb.Close()
	return nil
}

func execSql(db *sql.DB, sql string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(sql)
	if err != nil {
		return fmt.Errorf("执行sql失败: %w, sql: %s", err, sql)
	}
	return tx.Commit()

}
