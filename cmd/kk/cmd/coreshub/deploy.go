/*
Copyright 2020 The KubeSphere Authors.

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

package coreshub

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/options"
	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/pipelines"
)

type DeployOptions struct {
	CommonOptions       *options.CommonOptions
	ClusterCfgFile      string
	SecurityEnhancement bool
	DownloadCmd         string
	Artifact            string
	SkipCheckMd5        bool
	Force               bool
	AicpWorkDir         string
	FreePasswd          bool
}

func NewDeployOptions() *DeployOptions {
	return &DeployOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdDeploy creates a new deploy command
func NewCmdDeploy() *cobra.Command {
	o := NewDeployOptions()
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy coreshub to a cluster",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(cmd, args))
			util.CheckErr(o.Validate(cmd, args))
			util.CheckErr(o.Run())
		},
	}

	o.CommonOptions.AddCommonFlag(cmd)
	o.AddFlags(cmd)

	if err := completionSetting(cmd); err != nil {
		panic(fmt.Sprintf("Got error with the completion setting"))
	}
	return cmd
}

func (o *DeployOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.Artifact == "" {

	}

	return nil
}

func (o *DeployOptions) Validate(_ *cobra.Command, _ []string) error {
	if o.ClusterCfgFile == "" {
		return errors.New("missing required flag: --filename/-f")
	}
	return nil
}

func (o *DeployOptions) Run() error {
	arg := common.Argument{
		FilePath:            o.ClusterCfgFile,
		SecurityEnhancement: o.SecurityEnhancement,
		Debug:               o.CommonOptions.Verbose,
		IgnoreErr:           o.CommonOptions.IgnoreErr,
		SkipConfirmCheck:    o.CommonOptions.SkipConfirmCheck,
		Artifact:            o.Artifact,
		SkipCheckMd5:        o.SkipCheckMd5,
		Namespace:           o.CommonOptions.Namespace,
		Force:               o.Force,
		AicpWorkDir:         o.AicpWorkDir,
		FreePasswd:          o.FreePasswd,
	}

	return pipelines.CoresHubDeploy(arg, o.DownloadCmd)
}

func (o *DeployOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ClusterCfgFile, "filename", "f", "", "Path to a configuration file")
	cmd.Flags().BoolVarP(&o.SecurityEnhancement, "with-security-enhancement", "", false, "Security enhancement")
	cmd.Flags().StringVarP(&o.DownloadCmd, "download-cmd", "", "curl -L -o %s %s",
		`The user defined command to download the necessary binary files. The first param '%s' is output path, the second param '%s', is the URL`)
	cmd.Flags().StringVarP(&o.Artifact, "artifact", "a", "", "Path to a KubeKey artifact")
	cmd.Flags().BoolVarP(&o.SkipCheckMd5, "skip-check-md5", "", false, "skip check md5")
	cmd.Flags().BoolVarP(&o.Force, "force", "", false, "force to deploy")
	cmd.Flags().StringVarP(&o.AicpWorkDir, "aicp-dir", "", "", "Aicp work dir")
	cmd.Flags().BoolVarP(&o.FreePasswd, "free-passwd", "p", false, "set free passwd for all nodes")
}

func completionSetting(cmd *cobra.Command) (err error) {
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) (
		strings []string, directive cobra.ShellCompDirective) {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}

	return
}
