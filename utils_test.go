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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExist(t *testing.T) {
	t.Run("Test file exist check", func(t *testing.T) {
		assert.Equal(t, true, Exist("./testdata/desktop/dde-file-manager.desktop"))
		assert.Equal(t, false, Exist("./testdata/fndsajklfj/fjdskladhfil"))
	})
}

func TestSyncFile(t *testing.T) {
	t.Run("Test sync file", func(t *testing.T) {
		assert.Nil(t, syncFile("./testdata/desktop/dde-file-manager.desktop"))
	})
}

func TestGetDelayTime(t *testing.T) {
	t.Run("Test desktop file autostart delay time", func(t *testing.T) {
		delay, err := getDelayTime("./testdata/desktop/dde-file-manager.desktop")
		assert.Nil(t, err)
		assert.Equal(t, 10*time.Second, delay)
	})
}

func TestGetLightDMAutoLoginUser(t *testing.T) {
	t.Run("Test get LightDM AutoLogin User", func(t *testing.T) {
		assert.NotPanics(t, func() {
			getLightDMAutoLoginUser()
		})
	})
}
