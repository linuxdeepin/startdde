/*
 * Copyright (C) 2016 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadProxyChainsConfig(t *testing.T) {
	t.Run("Test load proxy chains config", func(t *testing.T) {
		cfg, err := loadProxyChainsConfig("./testdata/proxy/proxychains.json")
		assert.Nil(t, err)
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
