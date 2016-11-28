/**
 * Copyright (C) 2016 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestDBusExists(t *testing.T) {
	Convey("Test dbus whether exists", t, func() {
		So(isDBusDestExist("org.freedesktop.DBus"), ShouldEqual, true)
		So(isDBusDestExist("org.freedesktop.DBus111"), ShouldEqual, false)
	})
}

func TestStrInList(t *testing.T) {
	Convey("Test item whether in list", t, func() {
		var list = []string{
			"abc",
			"xyz",
			"123",
		}
		So(isItemInList("abc", list), ShouldEqual, true)
		So(isItemInList("abcd", list), ShouldEqual, false)
	})
}

func TestTaskInfo(t *testing.T) {
	Convey("Test task create", t, func() {
		So(newTaskInfo("test1", nil, nil), ShouldBeNil)
		So(newTaskInfo("test1",
			func() bool { return true },
			func() error { return nil }), ShouldNotBeNil)
	})

	task1 := newTaskInfo("test1",
		func() bool { return false },
		func() error { return nil })
	Convey("Test task state", t, func() {
		task1.Enable(false)
		So(task1.CanLaunch(), ShouldEqual, false)
		task1.Enable(true)
		So(task1.CanLaunch(), ShouldEqual, true)
		task1.failed = true
		So(task1.CanLaunch(), ShouldEqual, false)
		task1.failed = false
		task1.isRunning = func() bool { return true }
		So(task1.CanLaunch(), ShouldEqual, false)
	})

	task2 := newTaskInfo("test2",
		func() bool { return false },
		func() error { return nil })
	Convey("Test manager", t, func() {
		var m = &Manager{
			taskList: &taskInfos{task1},
		}
		So(m.IsTaskExist(task1.Name), ShouldEqual, true)
		So(m.IsTaskExist(task2.Name), ShouldEqual, false)
		task1.failed = true
		So(m.HasRunning(), ShouldEqual, false)
		*m.taskList = append(*m.taskList, task2)
		So(m.HasRunning(), ShouldEqual, true)
	})
}
