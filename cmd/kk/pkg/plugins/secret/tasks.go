package secret

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	yamlV3 "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/klog/v2"
)

type GenerateAicpKeysTask struct {
	common.KubeAction
}

func (i *GenerateAicpKeysTask) Execute(runtime connector.Runtime) error {
	cmName := filepath.Join(i.KubeConf.Arg.AicpWorkDir, common.AicpKeyCfg)
	klog.Infof("Starting GenerateAicpKeysTask, config file: %s", cmName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := make(map[string]string)

	// 先尝试读取现有文件
	if fileData, err := os.ReadFile(cmName); err == nil && len(fileData) > 0 {
		klog.Infof("Found existing key file, size: %d bytes", len(fileData))
		if err := yamlV3.Unmarshal(fileData, &keys); err != nil {
			klog.Warningf("Failed to parse existing key file: %v, will regenerate", err)
			keys = make(map[string]string)
		} else {
			klog.Infof("Successfully loaded %d keys from file", len(keys))
		}
	} else {
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read key file: %w", err)
		}
		klog.Infof("Key file does not exist or is empty, will create new keys")
	}

	// if keys is already generated, return
	if len(keys) == 14 {
		klog.Infof("Checking if all 14 keys are valid...")
		emptyValue := false
		for k, v := range keys {
			if v == "" {
				klog.Warningf("Key %s is empty", k)
				emptyValue = true
				break
			}
		}
		if !emptyValue {
			klog.Infof("All 14 keys are valid, using existing keys")
			i.PipelineCache.Set(common.IAAS_AKSK, keys)
			return nil
		}
		klog.Infof("Some keys are empty, regenerating...")
	} else {
		klog.Infof("Only found %d keys, need 14 keys, will generate missing ones", len(keys))
	}

	klog.Infof("Starting to generate/update keys...")
	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	pullImage := fmt.Sprintf("%s/aicp/encode-keys:v1", i.KubeConf.Cluster.Registry.PrivateRegistry)
	klog.Infof("Pulling docker image: %s", pullImage)
	err = pullDockerImage(ctx, dockerClient, pullImage)
	if err != nil {
		return fmt.Errorf("failed to pull docker image: %w", err)
	}
	klog.Infof("Docker image pulled successfully")

	keyFields := []string{
		common.ADMIN_KEY_ID,
		common.ADMIN_SECRET_KEY,
		common.CONSOLE_KEY_ID,
		common.CONSOLE_SECRET_KEY,
		common.BOSS_KEY_ID,
		common.BOSS_SECRET_KEY,
		common.REDIS_PASSWORD,
		common.PG_AICP,
		common.PG_YUNIFY,
	}

	klog.Infof("Generating basic keys...")
	for _, k := range keyFields {
		if _, ok := keys[k]; !ok {
			keys[k] = generateRandomString(k, keys)
			klog.Infof("Generated key: %s", k)
		} else {
			klog.Infof("Key %s already exists, skipping", k)
		}
	}

	encodeFields := map[string]string{
		common.ADMIN_SECRET_CONSOLE_KEY:   common.ADMIN_SECRET_KEY,
		common.CONSOLE_SECRET_CONSOLE_KEY: common.CONSOLE_SECRET_KEY,
		common.BOSS_SECRET_CONSOLE_KEY:    common.BOSS_SECRET_KEY,
		common.REDIS_ENCODE_PASSWORD:      common.REDIS_PASSWORD,
		common.PG_YUNIFY_ENCODE:           common.PG_YUNIFY,
	}

	klog.Infof("Encoding keys using Docker...")
	for k, v := range encodeFields {
		key, ok := keys[v]
		if !ok && key == "" {
			keys[v] = generateRandomString(k, keys)
			klog.Infof("Generated missing key for encoding: %s", v)
		}
		klog.Infof("Encoding key %s from source %s", k, v)
		dockerGenKey, err := runDockerAction(ctx, dockerClient, pullImage, []string{key})
		if err != nil {
			return fmt.Errorf("failed to encode key %s: %w", k, err)
		}
		klog.Infof("Successfully encoded key %s", k)
		keys[k] = dockerGenKey
	}

	// 写入文件
	klog.Infof("Writing keys to file: %s", cmName)
	yamlData, err := yamlV3.Marshal(keys)
	if err != nil {
		return fmt.Errorf("failed to marshal keys to YAML: %w", err)
	}

	err = os.WriteFile(cmName, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write keys to file: %w", err)
	}
	klog.Infof("Successfully wrote %d keys to file", len(keys))

	i.PipelineCache.Set(common.IAAS_AKSK, keys)
	klog.Infof("Keys saved to pipeline cache")

	return nil
}

func pullDockerImage(ctx context.Context, dockerClient *dockerclient.Client, image string) error {
	pull, err := dockerClient.ImagePull(ctx, image, dockerTypes.ImagePullOptions{})
	if err != nil {
		return err
	}
	io.Copy(io.Discard, pull)
	defer pull.Close()
	return nil
}

func runDockerAction(ctx context.Context, dockerClient *dockerclient.Client, image string, cmd []string) (string, error) {

	container, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   cmd,
	}, nil, nil, nil, "generate-key")
	if err != nil {
		return "", fmt.Errorf("create container failed: %w", err)
	}

	err = dockerClient.ContainerStart(ctx, container.ID, dockerTypes.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("start container failed: %w", err)
	}

	logs, err := dockerClient.ContainerLogs(ctx, container.ID, dockerTypes.ContainerLogsOptions{
		ShowStdout: true,
		Follow:     true,
	})
	if err != nil {
		return "", fmt.Errorf("get container logs failed: %w", err)
	}
	defer logs.Close()

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, logs)
	if err != nil {
		return "", fmt.Errorf("stdcopy failed: %w", err)
	}

	dockerClient.ContainerRemove(ctx, container.ID, dockerTypes.ContainerRemoveOptions{
		Force: true,
	})
	return strings.Trim(stdout.String(), "\n"), nil
}

func generateRandomString(keysTag string, keys map[string]string) string {
	key_id := ""
	if strings.HasSuffix(keysTag, "_KEY_ID") && len(keys[keysTag]) != 20 {
		letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		for i := 0; i < 20; i++ {
			idx := rand.Intn(len(letters))
			key_id += string(letters[idx])
		}
	} else if strings.HasSuffix(keysTag, "_SECRET_KEY") && len(keys[keysTag]) != 40 {
		letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012356789"
		for i := 0; i < 40; i++ {
			idx := rand.Intn(len(letters))
			key_id += string(letters[idx])
		}
	} else if len(keys[keysTag]) != 15 {
		key_id = rand.String(15)
	}
	klog.Infof("generate random string for %s: %s", keysTag, key_id)
	return key_id

}
