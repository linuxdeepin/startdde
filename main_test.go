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
	"github.com/linuxdeepin/go-lib/log"
)

func Test_ShouldUseDDEKWin(t *testing.T) {
	t.Run("Test is should use DDE KWin", func(t *testing.T) {
		should := shouldUseDDEKWin()
		exist := Exist("/usr/bin/kwin_no_scale")
		assert.Equal(t, exist, should)
	})
}

func Test_doSetLogLevel(t *testing.T) {
	doSetLogLevel(log.LevelDebug)
	assert.Equal(t, log.LevelDebug, logger.GetLogLevel())
}

func Test_loginReminder(t *testing.T) {
	tests := []struct {
		name            string
		gsLoginReminder bool
	}{
		{
			name:            "loginReminder disabled",
			gsLoginReminder: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_gSettingsConfig = &GSettingsConfig{
				loginReminder: tt.gsLoginReminder,
			}

			loginReminder()
		})
	}
}
