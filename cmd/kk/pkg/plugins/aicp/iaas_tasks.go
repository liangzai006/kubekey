package aicp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// global
type ProductManagerServerTask struct {
	common.KubeAction
}

func (p *ProductManagerServerTask) Execute(runtime connector.Runtime) error {
	productManagerServerDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "product-manager-server")
	iaasKeys, ok := p.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},

		"config": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_AICP,
				"password": iaasKeys.(map[string]string)[common.PG_AICP],
			},
		},
	}
	helm := HelmOptions{
		Name:      "product-manager-server",
		Namespace: "global-system",
		ChartPath: productManagerServerDir,
		Values:    vals,
		PreHook: func(ctx context.Context, kubeClient kubernetes.Interface) error {
			_, err := kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "global-system",
					Labels: map[string]string{
						"istio-injection": "enabled",
						"control-plane":   "global-system",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !apierror.IsAlreadyExists(err) {
				return err
			}

			return nil
		},
	}
	return helm.Install()
}

// global
type TeamTask struct {
	common.KubeAction
}

func (t *TeamTask) Execute(runtime connector.Runtime) error {
	teamDir := filepath.Join(t.KubeConf.Arg.AicpWorkDir, "charts", "team")
	iaasKeys, ok := t.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"repository": t.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_AICP,
				"password": iaasKeys.(map[string]string)[common.PG_AICP],
			},
		},
	}
	helm := HelmOptions{
		Name:      "team",
		Namespace: "global-system",
		ChartPath: teamDir,
		Values:    vals,
	}
	return helm.Install()
}

// global
type ImaasTask struct {
	common.KubeAction
}

func (i *ImaasTask) Execute(runtime connector.Runtime) error {
	imaasDir := filepath.Join(i.KubeConf.Arg.AicpWorkDir, "charts", "imaas")
	iaasKeys, ok := i.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"repository": i.KubeConf.Cluster.Registry.PrivateRegistry,
		},

		"config": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_AICP,
				"password": iaasKeys.(map[string]string)[common.PG_AICP],
			},
		},
	}
	helm := HelmOptions{
		Name:      "imaas",
		Namespace: "maas-system",
		ChartPath: imaasDir,
		Values:    vals,
	}
	return helm.Install()
}

type AccountTask struct {
	common.KubeAction
}

func (a *AccountTask) Execute(runtime connector.Runtime) error {
	accountDir := filepath.Join(a.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "account")
	iaasKeys, ok := a.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": a.KubeConf.Cluster.Registry.PrivateRegistry,
		},

		"configMap": map[string]interface{}{
			"zone":                       a.KubeConf.Cluster.Aicp.Zone,
			"domain":                     a.KubeConf.Cluster.Aicp.DomainConfig.Domain,
			"protocol":                   a.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
			"aiPrefix":                   a.KubeConf.Cluster.Aicp.DomainConfig.Ai,
			"ADMIN_KEY_ID":               iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
			"ADMIN_SECRET_KEY":           iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			"ADMIN_SECRET_CONSOLE_KEY":   iaasKeys.(map[string]string)[common.ADMIN_SECRET_CONSOLE_KEY],
			"CONSOLE_KEY_ID":             iaasKeys.(map[string]string)[common.CONSOLE_KEY_ID],
			"CONSOLE_SECRET_CONSOLE_KEY": iaasKeys.(map[string]string)[common.CONSOLE_SECRET_CONSOLE_KEY],
			"BOSS_KEY_ID":                iaasKeys.(map[string]string)[common.BOSS_KEY_ID],
			"BOSS_SECRET_CONSOLE_KEY":    iaasKeys.(map[string]string)[common.BOSS_SECRET_CONSOLE_KEY],
			"redis": map[string]interface{}{
				"password": iaasKeys.(map[string]string)[common.REDIS_ENCODE_PASSWORD],
			},
			"pg": map[string]interface{}{
				"user":            common.PG_YUNIFY,
				"password":        iaasKeys.(map[string]string)[common.PG_YUNIFY],
				"password_encode": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
			},
		},
		"memcached": map[string]interface{}{
			"image": map[string]interface{}{
				"registry": a.KubeConf.Cluster.Registry.PrivateRegistry,
			},
		},
	}
	helm := HelmOptions{
		Name:      "account",
		Namespace: "pitrix",
		ChartPath: accountDir,
		Values:    vals,
	}
	return helm.Install()
}

type ConsoleTask struct {
	common.KubeAction
}

func (c *ConsoleTask) Execute(runtime connector.Runtime) error {
	consoleDir := filepath.Join(c.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "console")
	iaasKeys, ok := c.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}

	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": c.KubeConf.Cluster.Registry.PrivateRegistry,
		},

		"configMap": map[string]interface{}{
			"iaas": map[string]interface{}{
				"access_key_id":     iaasKeys.(map[string]string)[common.CONSOLE_KEY_ID],
				"secret_access_key": iaasKeys.(map[string]string)[common.CONSOLE_SECRET_KEY],
				"default_zone":      c.KubeConf.Cluster.Aicp.Zone,
				"consolePrefix":     c.KubeConf.Cluster.Aicp.DomainConfig.Console,
				"protocol":          c.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
				"docsPrefix":        c.KubeConf.Cluster.Aicp.DomainConfig.Docs,
				"hostPrefix":        c.KubeConf.Cluster.Aicp.DomainConfig.Api,
			},
			"pg": map[string]interface{}{
				"user":            common.PG_YUNIFY,
				"password":        iaasKeys.(map[string]string)[common.PG_YUNIFY],
				"password_encode": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
			},
			"redis": map[string]interface{}{
				"password": iaasKeys.(map[string]string)[common.REDIS_ENCODE_PASSWORD],
			},
			"aicp": map[string]interface{}{
				"domain":   c.KubeConf.Cluster.Aicp.DomainConfig.Domain,
				"aiPrefix": c.KubeConf.Cluster.Aicp.DomainConfig.Ai,
			},
		},
	}
	helm := HelmOptions{
		Name:      "console",
		Namespace: "pitrix",
		ChartPath: consoleDir,
		Values:    vals,
	}
	return helm.Install()
}

type BossTask struct {
	common.KubeAction
}

func (b *BossTask) Execute(runtime connector.Runtime) error {
	bossDir := filepath.Join(b.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "boss")
	iaasKeys, ok := b.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"repository": b.KubeConf.Cluster.Registry.PrivateRegistry,
		"configMap": map[string]interface{}{
			"pg": map[string]interface{}{
				"user":     common.PG_YUNIFY,
				"password": iaasKeys.(map[string]string)[common.PG_YUNIFY],
			},
			"iaas": map[string]interface{}{
				"access_key_id":     iaasKeys.(map[string]string)[common.BOSS_KEY_ID],
				"secret_access_key": iaasKeys.(map[string]string)[common.BOSS_SECRET_KEY],
				"default_zone":      b.KubeConf.Cluster.Aicp.Zone,
				"domain":            b.KubeConf.Cluster.Aicp.DomainConfig.Domain,
				"hostPrefix":        b.KubeConf.Cluster.Aicp.DomainConfig.Api,
				"protocol":          b.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
			},
		},
	}
	helm := HelmOptions{
		Name:      "boss",
		Namespace: "pitrix",
		ChartPath: bossDir,
		Values:    vals,
	}
	return helm.Install()
}

type ProductTask struct {
	common.KubeAction
}

func (p *ProductTask) Execute(runtime connector.Runtime) error {
	productDir := filepath.Join(p.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "production")
	iaasKeys, ok := p.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": p.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"redis": map[string]interface{}{
			"Password": iaasKeys.(map[string]string)[common.REDIS_ENCODE_PASSWORD],
		},
		"pgsql": map[string]interface{}{
			"user":     common.PG_YUNIFY,
			"password": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
		},
	}
	helm := HelmOptions{
		Name:      "product",
		Namespace: "pitrix",
		ChartPath: productDir,
		Values:    vals,
	}
	return helm.Install()
}

type GlueTask struct {
	common.KubeAction
}

func (g *GlueTask) Execute(runtime connector.Runtime) error {
	glueDir := filepath.Join(g.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "glue")
	iaasKeys, ok := g.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": g.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"redis": map[string]interface{}{
			"Password": iaasKeys.(map[string]string)[common.REDIS_ENCODE_PASSWORD],
		},
		"pgsql": map[string]interface{}{
			"user":     common.PG_YUNIFY,
			"password": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
		},
		"config": map[string]interface{}{
			"ak": iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
			"sk": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
		},
	}
	helm := HelmOptions{
		Name:      "glue",
		Namespace: "pitrix",
		ChartPath: glueDir,
		Values:    vals,
	}
	return helm.Install()
}

type DocsTask struct {
	common.KubeAction
}

func (d *DocsTask) Execute(runtime connector.Runtime) error {
	docsDir := filepath.Join(d.KubeConf.Arg.AicpWorkDir, "charts", "docs")
	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": d.KubeConf.Cluster.Registry.PrivateRegistry,
		},
	}
	helm := HelmOptions{
		Name:      "docs",
		Namespace: "pitrix",
		ChartPath: docsDir,
		Values:    vals,
	}
	return helm.Install()
}

type BillingTask struct {
	common.KubeAction
}

func (b *BillingTask) Execute(runtime connector.Runtime) error {
	billingDir := filepath.Join(b.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "billing")
	iaasKeys, ok := b.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": b.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"pgsql": map[string]interface{}{
			"user":            common.PG_YUNIFY,
			"password":        iaasKeys.(map[string]string)[common.PG_YUNIFY],
			"password_encode": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
		},
		"qai": map[string]interface{}{
			"domain":   b.KubeConf.Cluster.Aicp.DomainConfig.Domain,
			"aiPrefix": b.KubeConf.Cluster.Aicp.DomainConfig.Ai,
			"protocol": b.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
			"ak":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
			"sk":       iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
		},
	}
	helm := HelmOptions{
		Name:      "billing",
		Namespace: "pitrix",
		ChartPath: billingDir,
		Values:    vals,
	}
	return helm.Install()
}

type WarehouseTask struct {
	common.KubeAction
}

func (w *WarehouseTask) Execute(runtime connector.Runtime) error {
	warehouseDir := filepath.Join(w.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "warehouse")
	iaasKeys, ok := w.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": w.KubeConf.Cluster.Registry.PrivateRegistry,
			"zone":        w.KubeConf.Cluster.Aicp.Zone,
		},
		"pgsql": map[string]interface{}{
			"user":     common.PG_YUNIFY,
			"passwd":   iaasKeys.(map[string]string)[common.PG_YUNIFY],
			"password": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
		},
	}
	helm := HelmOptions{
		Name:      "warehouse",
		Namespace: "pitrix",
		ChartPath: warehouseDir,
		Values:    vals,
	}
	return helm.Install()
}

type NginxTask struct {
	common.KubeAction
}

func (n *NginxTask) Execute(runtime connector.Runtime) error {
	nginxDir := filepath.Join(n.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "nginx")
	ssl, err := withSslConfig(n.KubeConf)
	if err != nil {
		return err
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": n.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"configMap": map[string]interface{}{
			"domain":        n.KubeConf.Cluster.Aicp.DomainConfig.Domain,
			"consolePrefix": n.KubeConf.Cluster.Aicp.DomainConfig.Console,
			"bossPrefix":    n.KubeConf.Cluster.Aicp.DomainConfig.Boss,
			"apiPrefix":     n.KubeConf.Cluster.Aicp.DomainConfig.Api,
			"cadminPrefix":  n.KubeConf.Cluster.Aicp.DomainConfig.Cadmin,
			"aiPrefix":      n.KubeConf.Cluster.Aicp.DomainConfig.Ai,
			"docsPrefix":    n.KubeConf.Cluster.Aicp.DomainConfig.Docs,
			"default_zone":  n.KubeConf.Cluster.Aicp.Zone,
			"zones": []interface{}{
				map[string]interface{}{
					"name":     n.KubeConf.Cluster.Aicp.Zone,
					"endpoint": "istio-ingressgateway.istio-system",
				},
			},
			"ssl": ssl,
		},
	}
	helm := HelmOptions{
		Name:      "nginx",
		Namespace: "pitrix",
		ChartPath: nginxDir,
		Values:    vals,
	}
	return helm.Install()
}

func withSslConfig(kubeConf *common.KubeConf) (map[string]interface{}, error) {
	sslConfig := map[string]interface{}{
		"enabled": false,
	}

	isSsl := kubeConf.Cluster.Aicp.DomainConfig.Protocol == "https"
	if !isSsl {
		return sslConfig, nil
	}
	k8sClient, err := utils.NewClient("")
	if err != nil {
		return nil, fmt.Errorf("create k8s client failed: %v", err)
	}

	generalCert, ok := kubeConf.Cluster.Aicp.DomainConfig.CertPaths[v1alpha2.GeneralCertKey]
	if !ok {
		return nil, errors.New("general cert not found")
	}
	secret, ok := createGeneralCert(generalCert.KeyFile, generalCert.CertFile)
	if !ok {
		return nil, errors.New("create general cert failed")
	}
	s, err := k8sClient.CoreV1().Secrets("pitrix").Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create general cert failed: %v", err)
	}
	sslConfig["useExistingSecret"] = true
	sslConfig["certs"] = map[string]interface{}{
		string(v1alpha2.ApiCertKey):     s.Name,
		string(v1alpha2.ConsoleCertKey): s.Name,
		string(v1alpha2.BossCertKey):    s.Name,
		string(v1alpha2.CadminCertKey):  s.Name,
		string(v1alpha2.AiCertKey):      s.Name,
		string(v1alpha2.DocsCertKey):    s.Name,
	}
	return sslConfig, nil

}

func createGeneralCert(KeyFile string, CertFile string) (*corev1.Secret, bool) {
	generalKey, err := os.ReadFile(KeyFile)
	if err != nil {
		klog.Errorf("read general key failed: %v", err)
		return nil, false
	}
	generalCert, err := os.ReadFile(CertFile)
	if err != nil {
		klog.Errorf("read general cert failed: %v", err)
		return nil, false
	}
	if _, err := tls.X509KeyPair(generalCert, generalKey); err != nil {
		klog.Errorf("read general cert failed: %v", err)
		return nil, false
	}
	secret := newSecretObj("general-tls-secret", "pitrix")
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       []byte(generalCert),
		corev1.TLSPrivateKeyKey: []byte(generalKey),
	}
	return secret, true
}

// newSecretObj will create a new Secret Object given name, namespace and secretType
func newSecretObj(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{},
	}
}

type MsgHubTask struct {
	common.KubeAction
}

func (m *MsgHubTask) Execute(runtime connector.Runtime) error {
	msgHubDir := filepath.Join(m.KubeConf.Arg.AicpWorkDir, "charts", "public-service", "msghub")
	iaasKeys, ok := m.PipelineCache.Get(common.IAAS_AKSK)
	if !ok {
		return fmt.Errorf(" get %s from pipeline cache failed", common.IAAS_AKSK)
	}
	vals := map[string]interface{}{
		"global": map[string]interface{}{
			"offlineRepo": m.KubeConf.Cluster.Registry.PrivateRegistry,
		},
		"config": map[string]interface{}{
			"domain": m.KubeConf.Cluster.Aicp.DomainConfig.Domain,
			"iaas": map[string]interface{}{
				"hostPrefix":      m.KubeConf.Cluster.Aicp.DomainConfig.Api,
				"protocol":        m.KubeConf.Cluster.Aicp.DomainConfig.Protocol,
				"accessKey":       iaasKeys.(map[string]string)[common.ADMIN_KEY_ID],
				"secretAccessKey": iaasKeys.(map[string]string)[common.ADMIN_SECRET_KEY],
			},
		},
		"pgsql": map[string]interface{}{
			"user":     common.PG_YUNIFY,
			"password": iaasKeys.(map[string]string)[common.PG_YUNIFY_ENCODE],
		},
		"redis": map[string]interface{}{
			"password": iaasKeys.(map[string]string)[common.REDIS_ENCODE_PASSWORD],
		},
	}
	helm := HelmOptions{
		Name:      "msghub",
		Namespace: "pitrix",
		ChartPath: msgHubDir,
		Values:    vals,
	}
	return helm.Install()
}
