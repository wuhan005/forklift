// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"time"

	"github.com/charmbracelet/log"
	"github.com/samber/lo"

	"github.com/wuhan005/forklift/internal/cri"
	"github.com/wuhan005/forklift/internal/k8s"
)

func main() {
	kubernetesServiceHost := flag.String("kubernetes-service-host", "https://kubernetes.default/", "Kubernetes service host")
	bearerTokenFile := flag.String("barer-token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Bearer token file")
	controllerServiceAddr := flag.String("controller-service-addr", "http://forklift-controller", "Controller service address")
	waitForReady := flag.Duration("wait-for-ready", 5*time.Second, "Wait for ready")
	flag.Parse()

	time.Sleep(*waitForReady)

	daemon, err := k8s.NewDaemon(k8s.NewDaemonOptions{
		KubernetesServiceHost: *kubernetesServiceHost,
		BearerTokenFile:       *bearerTokenFile,
		ControllerServiceAddr: *controllerServiceAddr,
	})
	if err != nil {
		panic(err)
	}

	for {
		log.Info("Start to watch images...")

		func() {
			ctx := context.Background()
			nodeImages, err := daemon.GetNodeImages(ctx)
			if err != nil {
				log.Error("Failed to get node images", "err", err)
				return
			}
			nodeImageNames := lo.Map(nodeImages, func(image *cri.Image, _ int) string {
				return image.Name
			})

			controllerImages, err := daemon.GetControllerImages(ctx)
			if err != nil {
				log.Error("Failed to get controller images", "err", err)
				return
			}

			_, pullImages := lo.Difference(nodeImageNames, controllerImages)
			if len(pullImages) == 0 {
				log.Info("No new images to pull")
			} else {
				log.Info("New images to pull", "images", pullImages)
				for _, image := range pullImages {
					if err := daemon.PullImage(ctx, image); err != nil {
						log.Error("Failed to pull image", "err", err)
					}
					log.Info("Pull images successfully", "image", image)
				}
			}
		}()

		time.Sleep(5 * time.Minute)
	}
}
