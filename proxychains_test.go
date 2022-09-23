// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadProxyChainsConfig(t *testing.T) {
	t.Run("Test load proxy chains config", func(t *testing.T) {
		cfg, err := loadProxyChainsConfig("./testdata/proxy/proxychains.json")
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "v2ray", cfg.Type)
		assert.Equal(t, "127.0.0.1", cfg.IP)
		assert.Equal(t, uint32(2333), cfg.Port)
		assert.Equal(t, "u123", cfg.User)
		assert.Equal(t, "p123", cfg.Password)
	})
}

func TestSupportProxyServerOption(t *testing.T) {
	t.Run("Test load proxy chains config", func(t *testing.T) {
		testdata := map[string]bool{
			"google-chrome-test": true,
			"browser360-test":    true,
			"uos-browser-test":   true,
			"chromium-test":      true,

			"test-google-chrome-test": false,
			"test-browser360-test":    false,
			"test-uos-browser-test":   false,
			"test-chromium-test":      false,
		}

		for k, v := range testdata {
			support := supportProxyServerOption(k)
			assert.Equal(t, v, support)
		}

	})
}
