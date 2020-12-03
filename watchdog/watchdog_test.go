/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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

package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDBusExists(t *testing.T) {
	t.Run("Test dbus whether exists", func(t *testing.T) {
		err := initDBusObject()
		if err != nil {
			logger.Warning("failed to init dbusObject:", err)
		}
		if busObj == nil {
			t.Skip("busObj is nil")
		}
		exist, _ := isDBusServiceExist(orgFreedesktopDBus)
		assert.True(t, exist)
		exist, _ = isDBusServiceExist(orgFreedesktopDBus + "111")
		assert.False(t, exist)
	})
}

func TestStrInList(t *testing.T) {
	t.Run("Test item whether in list", func(t *testing.T) {
		var list = []string{
			"abc",
			"xyz",
			"123",
		}
		assert.True(t, isItemInList("abc", list))
		assert.False(t, isItemInList("abcd", list))
	})
}

func TestTaskInfo(t *testing.T) {
	t.Run("Test task create", func(t *testing.T) {
		assert.Nil(t, newTaskInfo("test1", nil, nil))
		assert.NotNil(t, newTaskInfo("test1",
			func() (bool, error) { return true, nil },
			func() error { return nil }))
	})

	task1 := newTaskInfo("test1",
		func() (bool, error) { return false, nil },
		func() error { return nil })
	t.Run("Test task state", func(t *testing.T) {
		task1.Enable(false)
		assert.False(t, task1.CanLaunch())
		task1.Enable(true)
		assert.True(t, task1.CanLaunch())
		task1.failed = true
		assert.False(t, task1.CanLaunch())
		task1.failed = false
		task1.isRunning = func() (bool, error) { return true, nil }
		assert.False(t, task1.CanLaunch())
	})

	task2 := newTaskInfo("test2",
		func() (bool, error) { return false, nil },
		func() error { return nil })
	t.Run("Test manager", func(t *testing.T) {
		var m = &Manager{
			timedTasks: []*taskInfo{task1},
		}
		task1.failed = true
		assert.False(t, m.hasAnyRunnableTimedTask())
		m.timedTasks = append(m.timedTasks, task2)
		assert.True(t, m.hasAnyRunnableTimedTask())
	})
}
