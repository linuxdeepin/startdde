// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/linuxdeepin/go-lib/log"
)

func Test_doSetLogLevel(t *testing.T) {
	doSetLogLevel(log.LevelDebug)
	assert.Equal(t, log.LevelDebug, logger.GetLogLevel())
}
