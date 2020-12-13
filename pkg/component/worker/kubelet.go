/*
Copyright 2020 Mirantis, Inc.

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
package worker

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/avast/retry-go"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

// Kubelet is the component implementation to manage kubelet
type Kubelet struct {
	CRISocket           string
	EnableCloudProvider bool
	K0sVars             constant.CfgVars
	KubeletConfigClient *KubeletConfigClient
	LogLevel            string
	Profile             string
	dataDir             string
	supervisor          supervisor.Supervisor
	ClusterDNS          string
}

// Init extracts the needed binaries
func (k *Kubelet) Init() error {
	cmd := "kubelet"
	if runtime.GOOS == "windows" {
		cmd = "kubelet.exe"
	}
	err := assets.Stage(k.K0sVars.BinDir, cmd, constant.BinDirMode)
	if err != nil {
		panic(err)
		return err
	}

	k.dataDir = filepath.Join(k.K0sVars.DataDir, "kubelet")
	err = util.InitDirectory(k.dataDir, constant.DataDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.dataDir)
	}

	err = util.InitDirectory(k.K0sVars.KubeletVolumePluginDir, constant.KubeletVolumePluginDirMode)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", k.K0sVars.KubeletVolumePluginDir)
	}

	return nil
}

// Run runs kubelet
func (k *Kubelet) Run() error {
	cmd := "kubelet"

	if runtime.GOOS == "windows" {
		cmd = "kubelet.exe"
	}

	logrus.Info("Starting kubelet")
	kubeletConfigPath := filepath.Join(k.K0sVars.DataDir, "kubelet-config.yaml")
	// get the "real" resolv.conf file (in systemd-resolvd bases system,
	// this will return /run/systemd/resolve/resolv.conf
	resolvConfPath := resolvconf.Path()

	args := []string{
		fmt.Sprintf("--root-dir=%s", k.dataDir),
		fmt.Sprintf("--volume-plugin-dir=%s", k.K0sVars.KubeletVolumePluginDir),

		fmt.Sprintf("--config=%s", kubeletConfigPath),
		fmt.Sprintf("--bootstrap-kubeconfig=%s", k.K0sVars.KubeletBootstrapConfigPath),
		fmt.Sprintf("--kubeconfig=%s", k.K0sVars.KubeletAuthConfigPath),
		fmt.Sprintf("--v=%s", k.LogLevel),
		fmt.Sprintf("--resolv-conf=%s", resolvConfPath),
		"--kube-reserved-cgroup=system.slice",
		"--runtime-cgroups=/system.slice/containerd.service",
		"--kubelet-cgroups=/system.slice/containerd.service",
	}

	if runtime.GOOS == "windows" {
		node, err := getNodeName()
		if err != nil {
			return err
		}
		spew.Dump(node, k.ClusterDNS)
		args = append(args, "--cgroups-per-qos=false")
		args = append(args, "--enforce-node-allocatable=")
		args = append(args, "--pod-infra-container-image=kubeletwin/pause")
		args = append(args, "--network-plugin=cni")
		args = append(args, "--cni-bin-dir=C:\\k\\cni")
		args = append(args, "--cni-conf-dir=C:\\k\\cni\\config")
		args = append(args, "--hostname-override="+node)
		args = append(args, `--resolv-conf=`)
		args = append(args, "--cluster-dns="+k.ClusterDNS)
		args = append(args, "--cluster-domain=cluster.local")
		args = append(args, "--hairpin-mode=promiscuous-bridge")
		args = append(args, "--cert-dir=C:\\var\\lib\\k0s\\kubelet_certs")
	} else {
		args = append(args, "--cgroups-per-qos=true")
	}

	if k.CRISocket != "" {
		rtType, rtSock, err := splitRuntimeConfig(k.CRISocket)
		if err != nil {
			return err
		}
		args = append(args, fmt.Sprintf("--container-runtime=%s", rtType))
		shimPath := "unix:///var/run/dockershim.sock"
		if runtime.GOOS == "windows" {
			shimPath = "npipe:////./pipe/dockershim"
		}
		if rtType == "docker" {
			args = append(args, fmt.Sprintf("--docker-endpoint=%s", rtSock))
			// this endpoint is actually pointing to the one kubelet itself creates as the cri shim between itself and docker
			args = append(args, fmt.Sprintf("--container-runtime-endpoint=%s", shimPath))
		} else {
			args = append(args, fmt.Sprintf("--container-runtime-endpoint=%s", rtSock))
		}
	} else {
		args = append(args, "--container-runtime=remote")
		args = append(args, fmt.Sprintf("--container-runtime-endpoint=unix://%s", path.Join(k.K0sVars.RunDir, "containerd.sock")))
	}

	// We only support external providers
	if k.EnableCloudProvider {
		args = append(args, "--cloud-provider=external")
	}
	logrus.Infof("starting kubelet with args: %v", args)
	k.supervisor = supervisor.Supervisor{
		Name:    cmd,
		BinPath: assets.BinPath(cmd, k.K0sVars.BinDir),
		RunDir:  k.K0sVars.RunDir,
		DataDir: k.K0sVars.DataDir,
		Args:    args,
	}

	err := retry.Do(func() error {
		kubeletconfig, err := k.KubeletConfigClient.Get(k.Profile)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(kubeletConfigPath, []byte(kubeletconfig), constant.CertSecureMode)
		if err != nil {
			return errors.Wrap(err, "failed to write kubelet config to disk")
		}

		return nil
	})
	if err != nil {
		return err
	}

	k.supervisor.Supervise()

	return nil
}

// Stop stops kubelet
func (k *Kubelet) Stop() error {
	return k.supervisor.Stop()
}

// Health-check interface
func (k *Kubelet) Healthy() error { return nil }

func splitRuntimeConfig(rtConfig string) (string, string, error) {
	runtimeConfig := strings.SplitN(rtConfig, ":", 2)
	if len(runtimeConfig) != 2 {
		return "", "", fmt.Errorf("cannot parse CRI socket path")
	}
	runtimeType := runtimeConfig[0]
	runtimeSocket := runtimeConfig[1]
	if runtimeType != "docker" && runtimeType != "remote" {
		return "", "", fmt.Errorf("unknown runtime type %s, must be either of remote or docker", runtimeType)
	}

	return runtimeType, runtimeSocket, nil
}

func getNodeName() (string, error) {
	req, err := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/local-hostname", nil)
	if err != nil {
		panic(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return os.Hostname()
	}
	defer resp.Body.Close()
	h, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("can't read aws hostname: %v", err)
	}
	return string(h), nil
}
