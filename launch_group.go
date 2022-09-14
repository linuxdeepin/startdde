// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

type Cmd struct {
	Command string   `json:"Command"`
	Wait    bool     `json:"Wait"`
	Args    []string `json:"Args"`
}

type launchGroup struct {
	Priority uint32 `json:"Priority"`
	Group    []Cmd  `json:"Group"`
}

type launchGroups []*launchGroup

const (
	sysLaunchGroupFile  = "/usr/share/startdde/auto_launch.json"
	userLaunchGroupFile = "startdde/auto_launch.json"
)

func (infos launchGroups) Len() int {
	return len(infos)
}

func (infos launchGroups) Less(i, j int) bool {
	return infos[i].Priority > infos[j].Priority
}

func (infos launchGroups) Swap(i, j int) {
	infos[i], infos[j] = infos[j], infos[i]
}

func loadGroupFile() (launchGroups, error) {
	userFile := filepath.Join(basedir.GetUserConfigDir(), userLaunchGroupFile)
	infos, err := doLoadGroupFile(userFile)
	if err != nil {
		infos, err = doLoadGroupFile(sysLaunchGroupFile)
	}
	return infos, err
}

func doLoadGroupFile(filename string) (launchGroups, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var infos launchGroups
	err = json.Unmarshal(contents, &infos)
	if err != nil {
		return nil, err
	}
	return infos, nil
}
