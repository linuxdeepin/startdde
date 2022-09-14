// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenUuid(t *testing.T) {
	t.Run("Test generate uuid", func(t *testing.T) {
		uuid := genUuid()
		match, err := regexp.MatchString(`^[0-9a-f]+\-[0-9a-f]+\-[0-9a-f]+\-[0-9a-f]+\-[0-9a-f]+$`, uuid)
		assert.NoError(t, err)
		assert.True(t, match)
	})
}
