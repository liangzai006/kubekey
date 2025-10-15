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

package init

import (
	"net"

	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/options"
	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/pipelines"
	"github.com/spf13/cobra"
)

type InitOsOptions struct {
	CommonOptions      *options.CommonOptions
	ClusterCfgFile     string
	Artifact           string
	RepositoryServerIp net.IP
	SkipCheckMd5       bool
}

func NewInitOsOptions() *InitOsOptions {
	return &InitOsOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdInitOs creates a new init os command
func NewCmdInitOs() *cobra.Command {
	o := NewInitOsOptions()
	cmd := &cobra.Command{
		Use:   "os",
		Short: "Init operating system",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(cmd, args))
			util.CheckErr(o.Run())
		},
	}

	o.CommonOptions.AddCommonFlag(cmd)
	o.AddFlags(cmd)
	return cmd
}

func (o *InitOsOptions) Run() error {
	arg := common.Argument{
		FilePath:     o.ClusterCfgFile,
		Debug:        o.CommonOptions.Verbose,
		Artifact:     o.Artifact,
		RepositoryIp: o.RepositoryServerIp,
		SkipCheckMd5: o.SkipCheckMd5,
	}
	return pipelines.InitDependencies(arg)
}

func (o *InitOsOptions) Complete(_ *cobra.Command, _ []string) error {

	if o.RepositoryServerIp == nil {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					o.RepositoryServerIp = ipnet.IP
					return nil
				}
			}
		}
	}
	return nil
}

func (o *InitOsOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ClusterCfgFile, "filename", "f", "", "Path to a configuration file")
	cmd.Flags().StringVarP(&o.Artifact, "artifact", "a", "", "Path to a KubeKey artifact")
	cmd.Flags().IPVarP(&o.RepositoryServerIp, "repository-server-ip", "r", nil, "Repository server ip")
	cmd.Flags().BoolVarP(&o.SkipCheckMd5, "skip-check-md5", "", false, "Skip checking md5 of downloaded files")
}
