// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
		assert.NoError(t, err)
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
