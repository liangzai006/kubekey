package secret

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/task"
)

type GenerateKeysModule struct {
	common.KubeModule
	Skip bool
}

func (g *GenerateKeysModule) IsSkip() bool {
	return g.Skip
}

func (g *GenerateKeysModule) Init() {
	g.Name = "GenerateKeysModule"
	g.Desc = "Generate keys"

	generateKeys := &task.LocalTask{
		Name:   "Generate Aicp KeysTask",
		Desc:   "Generate Aicp Keys",
		Action: new(GenerateAicpKeysTask),
	}

	g.Tasks = []task.Interface{
		generateKeys,
	}
}
