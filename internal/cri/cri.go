// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cri

import (
	"context"
	"strings"
)

type Image struct {
	Name   string
	Digest string
}

type CRI interface {
	ListImages(ctx context.Context) ([]*Image, error)
	PullImage(ctx context.Context, image string) error
	LoadImage(ctx context.Context, image, sourcePath string) error
	ExportImage(ctx context.Context, image, destPath string) error
}

func ImageTarName(imageName string) string {
	return strings.ReplaceAll(imageName, "/", "-") + ".tar"
}
