package templates

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/lithammer/dedent"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/homedir"
)

var Cluster = template.Must(template.New("cluster").Parse(
	dedent.Dedent(`apiVersion: cluster.kubesphere.io/v1alpha1
kind: Cluster
spec:
  provider: kubesphere
  config: {{ .Config }}
  connection:
    type: direct
    kubeconfig: {{ .KubeConfig }}
  joinFederation: true
metadata:
  name: {{ .ClusterName }}
  labels:
    kubesphere.io/managed: "true"
`)))

var Config = template.Must(template.New("Config").Parse(
	dedent.Dedent(`global:
  imageRegistry: {{ .ImageRegistry }}
helmExecutor:
  image:
    registry: ""
    repository: kubesphere/kubectl
    tag: "v1.27.12"

kubectl:
  image:
    registry: ""
    repository: kubesphere/kubectl
    tag: "v1.27.12"
    pullPolicy: IfNotPresent
extension:
  imageRegistry: {{ .ImageRegistry }}
`)))

func GetKubeConfig(runtime *common.KubeConf) string {
	kubePath := path.Join(homedir.HomeDir(), ".kube", "config")
	file, err := os.ReadFile(kubePath)
	if err != nil {
		return ""
	}

	kubeConfigMap := map[string]interface{}{}

	err = yaml.NewDecoder(bytes.NewReader(file)).Decode(kubeConfigMap)

	if err != nil {
		return ""
	}

	serverAddress := runtime.Cluster.ControlPlaneEndpoint.Address
	serverPort := runtime.Cluster.ControlPlaneEndpoint.Port
	if serverAddress == "" {
		return ""
	}

	clusters := kubeConfigMap["clusters"].([]interface{})
	for _, clusterItem := range clusters {
		clusterMap := clusterItem.(map[string]interface{})
		if clusterData, ok := clusterMap["cluster"].(map[string]interface{}); ok {

			clusterData["server"] = fmt.Sprintf("https://%s:%d", serverAddress, serverPort)
		}
	}

	data, _ := yaml.Marshal(kubeConfigMap)

	return base64.StdEncoding.EncodeToString(data)
}
