/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

	"github.com/linuxdeepin/go-lib/locale"
)

// key is locale code
type defaultFontConfig map[string]fontConfigItem

type fontConfigItem struct {
	Standard  string
	Monospace string `json:"Mono"`
}

func (cfg defaultFontConfig) Get() (standard, monospace string) {
	languages := locale.GetLanguageNames()
	for _, lang := range languages {
		if item, ok := cfg[lang]; ok {
			return item.Standard, item.Monospace
		}
	}

	defaultItem := cfg["en_US"]
	return defaultItem.Standard, defaultItem.Monospace
}

func loadDefaultFontConfig(filename string) (defaultFontConfig, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var defaultFontConfig defaultFontConfig
	err = json.Unmarshal(contents, &defaultFontConfig)
	if err != nil {
		return nil, err
	}
	return defaultFontConfig, nil
}
