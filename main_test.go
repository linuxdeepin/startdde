// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
