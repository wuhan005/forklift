// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package k8s

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/wuhan005/forklift/internal/cri"
)

type Controller struct {
	k8sClient *kubernetes.Clientset
	config    *Config
	cri       cri.CRI
}

type NewControllerOptions struct {
	KubernetesServiceHost string
	BearerTokenFile       string
	ConfigFilePath        string
}

func NewController(options NewControllerOptions) (*Controller, error) {
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

	controller := &Controller{
		k8sClient: k8sClient,
	}
	if err := controller.LoadConfig(options.ConfigFilePath); err != nil {
		return nil, errors.Wrap(err, "load config")
	}

	ctx := context.Background()
	containerRuntime, _, err := GetContainerRuntime(ctx, k8sClient, "forklift-controller")
	if err != nil {
		return nil, errors.Wrap(err, "get container runtime")
	}

	switch containerRuntime {
	case "docker":
		controller.cri = cri.NewDocker(cri.NewDockerOptions{
			K8sClient: k8sClient,
			GetContainerID: func() (string, error) {
				_, containerID, err := GetContainerRuntime(ctx, k8sClient, "forklift-controller")
				return containerID, err
			},
		})
	case "containerd":
		// TODO
	default:
		return nil, errors.Errorf("unsupported container runtime %q", containerRuntime)
	}

	return controller, nil
}

func (c *Controller) GetImages(ctx context.Context) []string {
	images := make([]string, 0)

	for _, namespace := range c.config.ActiveNamespaces {
		pods, err := c.k8sClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{})
		if err != nil {
			log.FromContext(ctx).Error(err, "namespace", namespace)
			continue
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				images = append(images, container.Image)
			}
		}
	}

	return lo.Uniq(images)
}

func (c *Controller) LoadImage(ctx context.Context, imageName string) (io.ReadCloser, error) {
	images, err := c.cri.ListImages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list controller images")
	}

	if lo.ContainsBy(images, func(image *cri.Image) bool {
		return image.Name == imageName
	}) {
		log.FromContext(ctx).Info("image already exists", "image", imageName)
	} else {
		log.FromContext(ctx).Info("image does not exist, pulling...", "image", imageName)
		if err := c.cri.PullImage(ctx, imageName); err != nil {
			return nil, errors.Wrap(err, "pull image")
		}
	}

	destPath := os.TempDir()
	destFilePath := filepath.Join(destPath, cri.ImageTarName(imageName))
	if _, err := os.Stat(destFilePath); err != nil {
		if os.IsNotExist(err) {
			// Export image to tar file.
			if err := c.cri.ExportImage(ctx, imageName, destPath); err != nil {
				return nil, errors.Wrap(err, "export image")
			}
		} else {
			return nil, errors.Wrap(err, "stat image file")
		}
	}

	f, err := os.Open(destFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "open image file")
	}
	return f, nil
}
