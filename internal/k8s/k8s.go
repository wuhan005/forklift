// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package k8s

import (
	"context"
	"net/url"
	"os"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetContainerRuntime(ctx context.Context, k8sClient *kubernetes.Clientset, containerName string) (string, string, error) {
	currentPod, err := k8sClient.CoreV1().Pods(CurrentNamespace()).Get(ctx, PodName(), v1.GetOptions{})
	if err != nil {
		return "", "", errors.Wrap(err, "get current pod")
	}

	var hostContainerID string
	for i := len(currentPod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		containerStatus := currentPod.Status.ContainerStatuses[i]
		if containerStatus.Name == containerName && containerStatus.ContainerID != "" {
			hostContainerID = containerStatus.ContainerID
			break
		}
	}

	if hostContainerID == "" {
		return "", "", errors.New("empty host container id")
	}
	containerURL, err := url.Parse(hostContainerID)
	if err != nil {
		return "", "", errors.Wrap(err, "parse container url")
	}

	containerRuntime := containerURL.Scheme
	return containerRuntime, containerURL.Host, nil
}

func CurrentNamespace() string {
	namespace, _ := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	return string(namespace)
}

func PodName() string {
	return os.Getenv("HOSTNAME")
}
