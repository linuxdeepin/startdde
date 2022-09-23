// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sort"
	"testing"
	"time"
)

func TestMapDelayHandler(t *testing.T) {
	var list []string
	dh := newMapDelayHandler(100*time.Millisecond, func(name string) {
		list = append(list, name)
		t.Log(name)
	})

	delay := 10 * time.Millisecond
	time.Sleep(delay)
	dh.AddTask("a")
	time.Sleep(delay)
	dh.AddTask("a")
	time.Sleep(delay)
	dh.AddTask("a")
	time.Sleep(delay)
	dh.AddTask("b")
	time.Sleep(delay)

	time.Sleep(100 * time.Millisecond)
	sort.Strings(list)
	if len(list) == 2 &&
		list[0] == "a" && list[1] == "b" {
		t.Log("ok")
	} else {
		t.Error("failed")
	}
}
