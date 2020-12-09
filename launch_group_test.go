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
)

func TestLaunchGroup(t *testing.T) {
	t.Run("Test launch groups", func(t *testing.T) {
		lg, err := doLoadGroupFile("./testdata/auto_launch/auto_launch.json")
		assert.Nil(t, err)
		assert.Equal(t, 2, lg.Len())
		assert.True(t, lg.Less(0, 1))
		lg.Swap(0, 1)
		assert.False(t, lg.Less(0, 1))
	})
}
