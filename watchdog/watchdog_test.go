// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package watchdog

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/linuxdeepin/go-lib/log"
)

func isUseKwin() bool {
	_, err := os.Stat("/usr/bin/kwin_no_scale")
	return err == nil
}

func Test_Start(t *testing.T) {
	assert.NotPanics(t, func() {
		SetLogLevel(log.LevelDebug)

		_manager = newManager()

		_manager.AddTimedTask(newDdeDesktopTask())
		_manager.AddTimedTask(newDdePolkitAgent())
		_manager.AddDBusTask(ddeDockServiceName, newDdeDockTask())
		_manager.AddDBusTask(ddeShutdownServiceName, newDdeShutdownTask())
		if isUseKwin() {
			_manager.AddDBusTask(kWinServiceName, newDdeKWinTask())
		} else {
			_manager.AddDBusTask(wmServiceName, newWMTask())
		}

		manager := GetManager()
		assert.Equal(t, manager, _manager)
		assert.NotNil(t, manager.quit)

		manager.QuitLoop()
		assert.Nil(t, manager.quit)
	})
}

func Test_isDBusServiceExist(t *testing.T) {
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

func Test_isItemInList(t *testing.T) {
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

func Test_newTaskInfo(t *testing.T) {
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
