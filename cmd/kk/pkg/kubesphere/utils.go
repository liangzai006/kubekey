package kubesphere

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/ghodss/yaml"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/otiai10/copy"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func Load(path string) (*kubekeyapiv1alpha2.ExtensionKsbuilder, error) {
	tempDir, err := os.MkdirTemp("", "chart")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir) // nolint

	if err = WriteFilesToTempDir(path, tempDir); err != nil {
		return nil, err
	}

	metadata, err := kubekeyapiv1alpha2.LoadMetadata(tempDir)
	if err != nil {
		return nil, err
	}
	var extension kubekeyapiv1alpha2.ExtensionKsbuilder
	extension.Metadata = metadata

	chartMetadata, err := yaml.Marshal(metadata.ToChartYaml())
	if err != nil {
		return nil, err
	}

	if err = os.WriteFile(tempDir+"/Chart.yaml", chartMetadata, 0644); err != nil {
		return nil, err
	}

	if err = LoadApplicationClass(metadata.Name, tempDir); err != nil {
		return nil, err
	}

	ch, err := loader.LoadDir(tempDir)
	if err != nil {
		return nil, err
	}
	chartFilename, err := chartutil.Save(ch, tempDir)
	if err != nil {
		return nil, err
	}
	chartContent, err := os.ReadFile(chartFilename)
	if err != nil {
		return nil, err
	}

	extension.ChartData = chartContent
	return &extension, nil
}

func WriteFilesToTempDir(path, tempDir string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return copy.Copy(path, tempDir)
	}

	return nil
}

type ApplicationClass struct {
	ApplicationClassGroup string                     `json:"applicationClassGroup,omitempty"`
	Name                  string                     `json:"name,omitempty"`
	Provisioner           string                     `json:"provisioner,omitempty"`
	Parameters            map[string]string          `json:"parameters,omitempty"`
	AppVersion            string                     `json:"appVersion,omitempty"`
	PackageVersion        string                     `json:"packageVersion,omitempty"`
	Icon                  string                     `json:"icon,omitempty"`
	Description           kubekeyapiv1alpha2.Locales `json:"description,omitempty"`
	Maintainer            *chart.Maintainer          `json:"maintainer,omitempty"`
}

var applicationClassTmpl = template.Must(template.New("ApplicationClass").Funcs(sprig.FuncMap()).Parse(`
apiVersion: applicationclass.kubesphere.io/v1alpha1
kind: ApplicationClass
metadata:
  name: {{.Name}}-{{.PackageVersion}}
  labels:
    applicationclass.kubesphere.io/group: {{.ApplicationClassGroup}}
provisioner: {{.Provisioner | quote}}
parameters: {{.Parameters | toJson}}
spec:
  appVersion: {{.AppVersion | quote}}
  packageVersion: {{.PackageVersion | quote}}
  icon: {{.Icon | quote}}
  description: {{.Description | toJson}}
  maintainer: {{.Maintainer | toJson}}
`))

func LoadApplicationClass(name, tempDir string) error {
	var b bytes.Buffer
	defer func() {
		b.Reset()
	}()

	content, err := os.ReadFile(tempDir + "/applicationclass.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var appClass ApplicationClass
	if err = yaml.Unmarshal(content, &appClass); err != nil {
		return err
	}

	// Validate
	if len(appClass.Name) == 0 {
		return nil
	}

	filePath := filepath.Join(tempDir, "charts/applicationclass")
	if err = os.MkdirAll(filePath, 0644); err != nil {
		return err
	}

	c := &chart.Metadata{
		APIVersion: chart.APIVersionV2,
		Name:       appClass.Name,
		Version:    appClass.PackageVersion,
		AppVersion: appClass.AppVersion,
	}
	appClassChart, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	if err = os.WriteFile(filePath+"/Chart.yaml", appClassChart, 0644); err != nil {
		return err
	}
	if err = os.MkdirAll(filePath+"/templates", 0644); err != nil {
		return err
	}

	if appClass.Provisioner == "kubesphere.io/helm-application" {
		var cmName = fmt.Sprintf("application-%s-chart", name)
		appClass.Parameters = make(map[string]string)
		appClass.Parameters["configmap"] = cmName
		appClass.Parameters["namespace"] = "kubesphere-system"

		if err = copy.Copy(tempDir+"/application-package.yaml", filePath+"/templates/application-package.yaml"); err != nil {
			return err
		}
	}

	if err = applicationClassTmpl.Execute(&b, appClass); err != nil {
		return err
	}
	return os.WriteFile(filePath+"/templates/applicationclass.yaml", b.Bytes(), 0644)
}

func getStatusState(obj *unstructured.Unstructured) (bool, error) {

	clusters, ok, err := unstructured.NestedStringSlice(obj.Object, "spec", "clusterScheduling", "placement", "clusters")
	if err != nil {
		return false, err
	}
	if !ok {
		// 获取status.state字段
		state, _, err := unstructured.NestedString(obj.Object, "status", "state")
		if err != nil {
			return false, err
		}

		klog.Infof("wait for resource complete. resource: %s, state: %s", obj.GetName(), state)
		return state == "Installed", nil
	}

	var ready []string
	for _, cluster := range clusters {
		state, _, err := unstructured.NestedString(obj.Object, "status", "clusterSchedulingStatuses", cluster, "state")
		if err != nil {
			return false, err
		}
		klog.Infof("wait  for resource complete. cluster: %s, resource: %s, state: %s", cluster, obj.GetName(), state)
		if state == "Installed" {
			ready = append(ready, cluster)
		}
	}

	return len(ready) == len(clusters), nil
}

func WaitForResource(fn func() (bool, error), timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollImmediateUntil(1*time.Second, fn, ctx.Done())
}
