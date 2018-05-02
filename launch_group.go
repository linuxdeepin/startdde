/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"pkg.deepin.io/lib/xdg/basedir"
)

type launchGroup struct {
	Priority uint32 `json:"Priority"`
	Group    []struct {
		Command string   `json:"Command"`
		Wait    bool     `json:"Wait"`
		Args    []string `json:"Args"`
	} `json:"Group"`
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
