// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package k8s

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

type Config struct {
	ActiveNamespaces []string `yaml:"active-namespaces"`
}

func (c *Controller) LoadConfig(configFilePath string) error {
	f, err := os.Open(configFilePath)
	if err != nil {
		return errors.Wrap(err, "open config file")
	}
	defer func() { _ = f.Close() }()

	var config Config
	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		return errors.Wrap(err, "decode yaml")
	}
	c.config = &config
	return nil
}
