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
