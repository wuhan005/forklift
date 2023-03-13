// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package k8s

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/wuhan005/forklift/internal/cri"
)

type Daemon struct {
	k8sClient             *kubernetes.Clientset
	cri                   cri.CRI
	controllerServiceAddr string
}

type NewDaemonOptions struct {
	KubernetesServiceHost string
	BearerTokenFile       string
	ControllerServiceAddr string
}

func NewDaemon(options NewDaemonOptions) (*Daemon, error) {
	config := &rest.Config{
		Host: options.KubernetesServiceHost,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
		BearerTokenFile: options.BearerTokenFile,
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "new kubernetes client")
	}

	daemon := &Daemon{
		k8sClient:             k8sClient,
		controllerServiceAddr: strings.TrimRight(options.ControllerServiceAddr, "/"),
	}

	ctx := context.Background()
	containerRuntime, _, err := GetContainerRuntime(ctx, k8sClient, "forklift-daemonset")
	if err != nil {
		return nil, errors.Wrap(err, "get container runtime")
	}

	switch containerRuntime {
	case "docker":
		daemon.cri = cri.NewDocker(cri.NewDockerOptions{
			K8sClient: k8sClient,
			GetContainerID: func() (string, error) {
				_, containerID, err := GetContainerRuntime(ctx, k8sClient, "forklift-daemonset")
				return containerID, err
			},
		})
	case "containerd":
		// TODO
	default:
		return nil, errors.Errorf("unsupported container runtime %q", containerRuntime)
	}

	return daemon, nil
}

// GetNodeImages return the image list on the node.
func (c *Daemon) GetNodeImages(ctx context.Context) ([]*cri.Image, error) {
	return c.cri.ListImages(ctx)
}

func (c *Daemon) GetControllerImages(_ context.Context) ([]string, error) {
	resp, err := http.Get(c.controllerServiceAddr)
	if err != nil {
		return nil, errors.Wrap(err, "get controller images")
	}
	defer func() { _ = resp.Body.Close() }()

	var images []string
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, errors.Wrap(err, "decode response")
	}
	return images, nil
}

func (c *Daemon) PullImage(ctx context.Context, imageName string) error {
	resp, err := http.Get(c.controllerServiceAddr + "/load?image=" + imageName)
	if err != nil {
		return errors.Wrap(err, "get controller images")
	}
	defer func() { _ = resp.Body.Close() }()

	imageFilePath := filepath.Join(os.TempDir(), cri.ImageTarName(imageName))
	f, err := os.Create(imageFilePath)
	if err != nil {
		return errors.Wrap(err, "open image file")
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return errors.Wrap(err, "io copy")
	}

	if err := c.cri.LoadImage(ctx, imageName, imageFilePath); err != nil {
		return errors.Wrap(err, "load image")
	}
	return nil
}
