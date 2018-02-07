/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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
