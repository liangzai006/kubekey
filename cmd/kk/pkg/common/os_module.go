package common

import (
	"os"
	"path/filepath"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"k8s.io/klog/v2"
)

type StepOSModule struct {
	KubeAction
	Step string
}

func (s *StepOSModule) Execute(runtime connector.Runtime) error {

	stepDir := filepath.Join(runtime.GetWorkDir(), "step")
	_, err := os.Stat(stepDir)
	if err != nil {
		os.MkdirAll(stepDir, 0775)
	}

	_, err = os.Create(filepath.Join(stepDir, s.Step))
	if err != nil {
		klog.Errorf("create step skip file: %s", filepath.Join(stepDir, s.Step))
	}

	return nil
}
