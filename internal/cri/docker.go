// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cri

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var _ CRI = (*Docker)(nil)

type Docker struct {
	k8sClient      *kubernetes.Clientset
	getContainerID func() (string, error)
}

type NewDockerOptions struct {
	K8sClient      *kubernetes.Clientset
	GetContainerID func() (string, error)
}

func NewDocker(options NewDockerOptions) *Docker {
	return &Docker{
		k8sClient:      options.K8sClient,
		getContainerID: options.GetContainerID,
	}
}

func (d *Docker) ListImages(ctx context.Context) ([]*Image, error) {
	execCommand := []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker images --format '{{.Repository}}:{{.Tag}} {{.Digest}}'",
	}

	output, err := exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Output()
	if err != nil {
		return nil, errors.Wrap(err, "exec docker images command")
	}

	images := make([]*Image, 0)

	for _, line := range bytes.Split(output, []byte{'\n'}) {
		groups := bytes.Split(line, []byte{' '})
		if len(groups) != 2 {
			continue
		}

		imageName := string(groups[0])
		imageDigest := string(groups[1]) // FIXME: the docker load command will not return the digest.

		image := &Image{
			Name:   imageName,
			Digest: imageDigest,
		}
		images = append(images, image)
	}
	return images, nil
}

func (d *Docker) PullImage(ctx context.Context, image string) error {
	execCommand := []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker pull " + image,
	}
	return exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Run()
}

func (d *Docker) ExportImage(ctx context.Context, imageName, destPath string) error {
	imageName = strings.TrimSpace(imageName)

	// Export image to node.
	tempFilePath := filepath.Join(os.TempDir(), ImageTarName(imageName))
	execCommand := []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker save -o " + tempFilePath + " " + imageName,
	}
	if err := exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Run(); err != nil {
		return errors.Wrap(err, "docker save")
	}

	// Move image file into container.
	containerID, err := d.getContainerID()
	if err != nil {
		return errors.Wrap(err, "get container id")
	}
	destContainerPath := containerID + ":" + destPath
	execCommand = []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker cp " + tempFilePath + " " + destContainerPath,
	}
	if err := exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Run(); err != nil {
		return errors.Wrap(err, "docker cp")
	}
	return nil
}

func (d *Docker) LoadImage(ctx context.Context, imageName, sourcePath string) error {
	imageName = strings.TrimSpace(imageName)

	// Move image file into node.
	containerID, err := d.getContainerID()
	if err != nil {
		return errors.Wrap(err, "get container id")
	}
	sourceContainerPath := containerID + ":" + sourcePath
	tempDestPath := os.TempDir()
	execCommand := []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker cp " + sourceContainerPath + " " + tempDestPath,
	}
	if err := exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Run(); err != nil {
		return errors.Wrap(err, "docker cp")
	}

	// Load image from node.
	tempFilePath := filepath.Join(tempDestPath, ImageTarName(imageName))
	execCommand = []string{
		"sh", "-c",
		"nsenter -t 1 -m -u -n -i docker load -i " + tempFilePath,
	}
	if err := exec.CommandContext(ctx, execCommand[0], execCommand[1:]...).Run(); err != nil {
		return errors.Wrap(err, "docker load")
	}
	return nil
}
