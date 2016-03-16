/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

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
