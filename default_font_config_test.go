// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFontConfig(t *testing.T) {
	t.Run("Test Font Config", func(t *testing.T) {
		testdata := map[string]fontConfigItem{
			"zh_CN": {"Noto Sans CJK SC", "Noto Mono"},
			"zh_TW": {"Noto Sans CJK TC", "Noto Mono"},
			"zh_HK": {"Noto Sans CJK TC", "Noto Mono"},
			"ja":    {"Noto Sans CJK JP", "Noto Mono"},
			"ko":    {"Noto Sans CJK KR", "Noto Mono"},
			"en_US": {"Noto Sans", "Noto Mono"},
		}

		cfg, err := loadDefaultFontConfig("./testdata/fontconfig/fontconfig.json")
		assert.NoError(t, err)

		for k, v := range testdata {
			assert.Equal(t, v, cfg[k])
		}
	})
}
