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
	"fmt"
	"os"
	"testing"
)

func _TestSetAutostart(t *testing.T) {
	m := StartManager{}
	if err := m.setAutostart("dropbox.desktop", true); err != nil {
		fmt.Println(err)
	}
	if !m.isAutostart("dropbox.desktop") {
		t.Error("set to autostart failed")
	}
	if err := m.setAutostart("dropbox.desktop", false); err != nil {
		fmt.Println(err)
	}
	if m.isAutostart("dropbox.desktop") {
		t.Error("set to not autostart failed")
	}
}

func _TestScanDir(t *testing.T) {
	scanDir("/tmp", func(p string, info os.FileInfo) bool {
		t.Log(info.Name())
		return false
	})
}
