// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultSinkAlsaDevice(t *testing.T) {
	t.Run("Test Get Default Sink Alsa Device", func(t *testing.T) {
		dev, _, err := getDefaultSinkAlsaDevice()
		if err == nil {
			match, _ := regexp.MatchString(`^plughw:CARD=\d+,DEV=\d+,SUBDEV=\d+$`, dev)
			assert.True(t, match)
		}
	})
}
