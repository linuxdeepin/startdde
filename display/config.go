/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package display

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"pkg.deepin.io/lib/strv"
)

type configMonitor struct {
	Name      string
	Primary   string
	BaseInfos MonitorBaseInfos
}

type configManager struct {
	BaseGroup map[string]*configMonitor
	filename  string
	rwLock    sync.RWMutex
}

func (config *configManager) get(id string) *configMonitor {
	config.rwLock.RLock()
	defer config.rwLock.RUnlock()
	if len(id) == 0 {
		return nil
	}

	infos, ok := config.BaseGroup[id]
	if !ok {
		return nil
	}
	return infos
}

func (config *configManager) set(id string, infos *configMonitor) {
	config.rwLock.Lock()
	defer config.rwLock.Unlock()
	if infos == nil {
		return
	}
	cur, ok := config.BaseGroup[id]
	if ok && cur.String() == infos.String() {
		return
	}
	config.BaseGroup[id] = infos
}

func (config *configManager) delete(id string) bool {
	config.rwLock.Lock()
	defer config.rwLock.Unlock()
	_, ok := config.BaseGroup[id]
	if !ok {
		return false
	}
	delete(config.BaseGroup, id)
	return true
}

func (config *configManager) writeFile() error {
	config.rwLock.Lock()
	defer config.rwLock.Unlock()
	data, err := json.Marshal(config.BaseGroup)
	if err != nil {
		return err
	}

	srcData, _ := ioutil.ReadFile(config.filename)
	if string(srcData) == string(data) {
		return nil
	}

	err = os.MkdirAll(path.Dir(config.filename), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(config.filename, data, 0644)
}

func (config *configManager) getIdList() map[string]string {
	var set = make(map[string]string)
	for k, v := range config.BaseGroup {
		set[k] = v.Name
	}
	return set
}

func (config *configManager) String() string {
	data, _ := json.Marshal(config)
	return string(data)
}

func (m *configMonitor) String() string {
	data, _ := json.Marshal(m)
	return string(data)
}

func newConfigManagerFromFile(filename string) (*configManager, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config = configManager{
		BaseGroup: make(map[string]*configMonitor),
		filename:  filename,
	}
	var baseGroup map[string]*configMonitor
	err = json.Unmarshal(data, &baseGroup)
	if err != nil {
		return nil, err
	}

	// fix baseGroup keys
	for key, value := range baseGroup {
		cfgKey := parseConfigKey(key)
		config.BaseGroup[cfgKey.String()] = value
	}

	return &config, nil
}

type configKey struct {
	name     string
	idFields []string
}

func parseConfigKey(str string) *configKey {
	var name string
	var idFields []string
	idx := strings.LastIndex(str, customModeDelim)
	if idx == -1 {
		idFields = strings.Split(str, monitorsIdDelimiter)
	} else {
		name = str[:idx]
		idFields = strings.Split(str[idx+1:], monitorsIdDelimiter)
	}

	sort.Strings(idFields)
	return &configKey{
		name:     name,
		idFields: idFields,
	}
}

func (ck *configKey) String() string {
	if ck.name == "" {
		return strings.Join(ck.idFields, monitorsIdDelimiter)
	}
	return ck.name + customModeDelim + strings.Join(ck.idFields, monitorsIdDelimiter)
}

func (ck *configKey) matchMonitorsId(id string) bool {
	idFields := strings.Split(id, monitorsIdDelimiter)
	return strv.Strv(idFields).Equal(ck.idFields)
}
