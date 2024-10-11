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

package os

import (
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	"io"
	"os"
	"path/filepath"
)

const (
	Release = "release"
)

func CopyFile(src, dst string) error {

	var (
		srcMd5, dstMd5 string
	)
	srcMd5 = util.LocalMd5Sum(src)

	if util.IsExist(dst) {
		dstMd5 = util.LocalMd5Sum(dst)
		if srcMd5 == dstMd5 {
			logger.Log.Debug("remote file %s md5 value is the same as local file, skip scp", dst)
			return nil
		}
	} else {
		dir := filepath.Dir(dst)
		err := os.MkdirAll(dir, os.FileMode(0777))
		if err != nil {
			return err
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}
