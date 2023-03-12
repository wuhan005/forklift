// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/wuhan005/forklift/internal/k8s"
)

func main() {
	kubernetesServiceHost := flag.String("kubernetes-service-host", "https://kubernetes.default/", "Kubernetes service host")
	bearerTokenFile := flag.String("barer-token-file", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Bearer token file")
	configFilePath := flag.String("config-file-path", "/etc/forklift/forklift.yaml", "Config file path")
	listenAddr := flag.String("listen-addr", ":80", "Listen address")
	flag.Parse()

	controller, err := k8s.NewController(k8s.NewControllerOptions{
		KubernetesServiceHost: *kubernetesServiceHost,
		BearerTokenFile:       *bearerTokenFile,
		ConfigFilePath:        *configFilePath,
	})
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		images := controller.GetImages(r.Context())
		_ = json.NewEncoder(w).Encode(images)
	})

	mux.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		images := controller.GetImages(r.Context())

		imageName := r.URL.Query().Get("image")
		if !lo.Contains(images, imageName) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		imageTar, err := controller.LoadImage(r.Context(), imageName)
		if err != nil {
			log.Error("Failed to load image", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer func() { _ = imageTar.Close() }()

		if _, err := io.Copy(w, imageTar); err != nil {
			log.Error("Failed to copy image", "err", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if err := http.ListenAndServe(*listenAddr, mux); err != nil {
		panic(err)
	}
}
