// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package memchecker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetConfig(t *testing.T) {
	config := GetConfig()
	assert.NotNil(t, config)
}

func Test_IsSufficient(t *testing.T) {
	assert.NotPanics(t, func() {
		IsSufficient()
	})
}
