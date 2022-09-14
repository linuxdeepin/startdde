// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
