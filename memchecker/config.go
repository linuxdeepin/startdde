// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package memchecker

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	sysConfigFile = "/usr/share/startdde/memchecker.json"

	defaultMinMemAvail = 300  // 300M
	defaultMaxSwapUsed = 1200 // 1200M
)

type configInfo struct {
	MinMemAvail uint64 `json:"min-mem-available"`
	MaxSwapUsed uint64 `json:"max-swap-used"`
}

func loadConfig(filename string) (*configInfo, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		content, err = ioutil.ReadFile(sysConfigFile)
		if err != nil {
			return nil, err
		}
	}

	var info configInfo
	err = json.Unmarshal(content, &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func getConfigPath() string {
	return filepath.Join(basedir.GetUserConfigDir(),
		"deepin", "startdde", "memchecker.json")
}
