/*
 Copyright 2021 The KubeSphere Authors.

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

package common

import (
	kubekeyv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/module"
)

type ArtifactManifest struct {
	Spec *kubekeyv1alpha2.ManifestSpec
	Arg  ArtifactArgument
}

type ArtifactModule struct {
	module.BaseTaskModule
	Manifest *ArtifactManifest
}

func (a *ArtifactModule) IsSkip() bool {
	return a.Skip
}

func (a *ArtifactModule) AutoAssert() {
	artifactRuntime := a.Runtime.(*ArtifactRuntime)
	m := &ArtifactManifest{
		Spec: artifactRuntime.Spec,
		Arg:  artifactRuntime.Arg,
	}

	a.Manifest = m
}
