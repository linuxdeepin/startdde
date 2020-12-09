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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClearTtys(t *testing.T) {
	t.Run("Test clear TTYs", func(t *testing.T) {
		assert.NotPanics(t, clearTtys)
	})
}

func TestSetupEnvironments1(t *testing.T) {
	t.Run("Test setup environments", func(t *testing.T) {
		setupEnvironments1()

		testdata := map[string]string{
			"GNOME_DESKTOP_SESSION_ID":         "this-is-deprecated",
			"XDG_CURRENT_DESKTOP":              "Deepin",
			"QT_LINUX_ACCESSIBILITY_ALWAYS_ON": "1",
		}

		for k, v := range testdata {
			assert.Equal(t, v, os.Getenv(k))
		}
	})
}
