package v1alpha2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"helm.sh/helm/v3/pkg/chart"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KubeSphereSystem  = "kubesphere-system"
	ConfigMapDataKey  = "chart.tgz"
	KubeSphereManaged = "kubesphere.io/managed"
)

type ExtensionKsbuilder struct {
	Metadata *Metadata
	// ChartURL valid when the chart source online.
	ChartURL string
	// ChartData valid when the chart source local.
	ChartData []byte
}

func (ext *ExtensionKsbuilder) ToKubernetesResources() []runtimeclient.Object {
	var resources = []runtimeclient.Object{
		&Extension{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kubesphere.io/v1alpha1",
				Kind:       "Extension",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ext.Metadata.Name,
				Labels: map[string]string{
					CategoryLabel:     ext.Metadata.Category,
					KubeSphereManaged: "true",
				},
			},
			Spec: ExtensionSpec{
				ExtensionInfo: ExtensionInfo{
					Description: ext.Metadata.Description,
					DisplayName: ext.Metadata.DisplayName,
					Icon:        ext.Metadata.Icon,
					Provider:    ext.Metadata.Provider,
					Created:     metav1.Now(),
				},
			},
			Status: ExtensionStatus{
				RecommendedVersion: ext.Metadata.Version,
			},
		},
	}

	extensionVersion := &ExtensionVersion{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubesphere.io/v1alpha1",
			Kind:       "ExtensionVersion",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", ext.Metadata.Name, ext.Metadata.Version),
			Labels: map[string]string{
				ExtensionReferenceLabel: ext.Metadata.Name,
				CategoryLabel:           ext.Metadata.Category,
			},
			Annotations: ext.Metadata.Annotations,
		},
		Spec: ExtensionVersionSpec{
			InstallationMode: ext.Metadata.InstallationMode,
			ExtensionInfo: ExtensionInfo{
				Description: ext.Metadata.Description,
				DisplayName: ext.Metadata.DisplayName,
				Icon:        ext.Metadata.Icon,
				Provider:    ext.Metadata.Provider,
				Created:     metav1.Now(),
			},
			Docs:                 ext.Metadata.Docs,
			Namespace:            ext.Metadata.Namespace,
			Home:                 ext.Metadata.Home,
			Keywords:             ext.Metadata.Keywords,
			KSVersion:            ext.Metadata.KSVersion,
			KubeVersion:          ext.Metadata.KubeVersion,
			Sources:              ext.Metadata.Sources,
			Version:              ext.Metadata.Version,
			Category:             ext.Metadata.Category,
			Screenshots:          ext.Metadata.Screenshots,
			ExternalDependencies: ext.Metadata.ExternalDependencies,
		},
	}

	if ext.ChartURL != "" {
		extensionVersion.Spec.ChartURL = ext.ChartURL
		resources = append(resources, extensionVersion)
	} else {
		configmap := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("extension-%s-%s-chart", ext.Metadata.Name, ext.Metadata.Version),
				Namespace: KubeSphereSystem,
			},
			BinaryData: map[string][]byte{
				ConfigMapDataKey: ext.ChartData,
			},
		}
		extensionVersion.Spec.ChartDataRef = &ConfigMapKeyRef{
			Namespace: configmap.Namespace,
			ConfigMapKeySelector: corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configmap.Name,
				},
				Key: ConfigMapDataKey,
			},
		}
		resources = append(resources, extensionVersion, configmap)
	}
	return resources
}

const MetadataFilename = "extension.yaml"

type Metadata struct {
	APIVersion string `json:"apiVersion" validate:"required"`
	// The name of the chart. Required.
	Name                 string                     `json:"name" validate:"required"`
	Version              string                     `json:"version" validate:"required"`
	DisplayName          Locales                    `json:"displayName" validate:"required"`
	Description          Locales                    `json:"description" validate:"required"`
	Category             string                     `json:"category" validate:"required"`
	Keywords             []string                   `json:"keywords,omitempty"`
	Home                 string                     `json:"home,omitempty"`
	Docs                 string                     `json:"docs,omitempty"`
	Sources              []string                   `json:"sources,omitempty"`
	KubeVersion          string                     `json:"kubeVersion,omitempty"`
	KSVersion            string                     `json:"ksVersion,omitempty"`
	Maintainers          []*chart.Maintainer        `json:"maintainers,omitempty"`
	Provider             map[LanguageCode]*Provider `json:"provider" validate:"required"`
	StaticFileDirectory  string                     `json:"staticFileDirectory,omitempty"`
	Icon                 string                     `json:"icon" validate:"required"`
	Screenshots          []string                   `json:"screenshots,omitempty"`
	Dependencies         []*chart.Dependency        `json:"dependencies,omitempty"`
	InstallationMode     InstallationMode           `json:"installationMode,omitempty"`
	Namespace            string                     `json:"namespace,omitempty"`
	Images               []string                   `json:"images,omitempty"`
	ExternalDependencies []ExternalDependency       `json:"externalDependencies,omitempty"`
	Annotations          map[string]string          `json:"annotations,omitempty"`
}

type Options struct {
	encodeIcon bool
}

func WithEncodeIcon(encodeIcon bool) func(opts *Options) {
	return func(opts *Options) {
		opts.encodeIcon = encodeIcon
	}
}

func LoadMetadata(path string, options ...func(*Options)) (*Metadata, error) {
	opts := &Options{
		encodeIcon: true,
	}
	for _, f := range options {
		f(opts)
	}

	content, err := os.ReadFile(filepath.Join(path, MetadataFilename))
	if err != nil {
		return nil, err
	}
	metadata, err := ParseMetadata(content)
	if err != nil {
		return nil, err
	}

	if IsLocalFile(metadata.Icon) && opts.encodeIcon {
		base64EncodedIcon, err := encodeIcon(filepath.Join(path, metadata.Icon))
		if err != nil {
			return nil, err
		}
		metadata.Icon = base64EncodedIcon
	}

	return metadata, nil
}

func ParseMetadata(data []byte) (*Metadata, error) {
	metadata := new(Metadata)
	if err := yaml.Unmarshal(data, metadata); err != nil {
		return nil, err
	}

	// set default value for necessary fields
	if metadata.InstallationMode == "" {
		metadata.InstallationMode = InstallationModeHostOnly
	}
	return metadata, nil
}

func (md *Metadata) ToChartYaml() *chart.Metadata {
	var c = chart.Metadata{
		APIVersion:   chart.APIVersionV2,
		Name:         md.Name,
		Version:      md.Version,
		Keywords:     md.Keywords,
		Sources:      md.Sources,
		KubeVersion:  md.KubeVersion,
		Home:         md.Home,
		Dependencies: md.Dependencies,
		Description:  string(md.Description[DefaultLanguageCode]),
		Icon:         md.Icon,
		Maintainers:  md.Maintainers,
		Annotations:  md.Annotations,
	}
	return &c
}

func DeepCopy(md *chart.Metadata) *chart.Metadata {
	data, _ := json.Marshal(md)
	out := &chart.Metadata{}
	_ = json.Unmarshal(data, out)
	return out
}

func IsLocalFile(path string) bool {
	if strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "data:image") {
		return false
	}
	return true
}

func encodeIcon(iconPath string) (string, error) {
	content, err := os.ReadFile(iconPath)
	if err != nil {
		return "", err
	}
	var base64Encoding string

	mimeType := mime.TypeByExtension(filepath.Ext(iconPath))
	if mimeType == "" {
		mimeType = http.DetectContentType(content)
	}

	base64Encoding += "data:" + mimeType + ";base64,"
	base64Encoding += base64.StdEncoding.EncodeToString(content)
	return base64Encoding, nil
}
