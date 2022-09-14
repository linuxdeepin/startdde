// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

type ProxyChainsConfig struct {
	Type     string
	IP       string
	Port     uint32
	User     string
	Password string
}

func loadProxyChainsConfig(file string) (*ProxyChainsConfig, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var cfg ProxyChainsConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func getProxyServerUrl() (string, error) {
	file := filepath.Join(basedir.GetUserConfigDir(), "deepin/proxychains.json")
	cfg, err := loadProxyChainsConfig(file)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s:%d", cfg.Type, cfg.IP, cfg.Port), nil
}

func supportProxyServerOption(appId string) bool {
	return strings.HasPrefix(appId, "google-chrome") ||
		strings.HasPrefix(appId, "browser360") ||
		strings.HasPrefix(appId, "uos-browser") ||
		strings.HasPrefix(appId, "chromium") ||
		strings.HasPrefix(appId,"org.deepin.browser")
}
