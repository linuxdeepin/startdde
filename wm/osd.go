/*
 * Copyright (C) 2017 ~ 2017 Deepin Technology Co., Ltd.
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

package wm

import (
	"pkg.deepin.io/lib/dbus"
)

func showOSD(name string) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	osd := conn.Object("com.deepin.dde.osd", "/")
	var r dbus.Variant
	err = osd.Call("com.deepin.dde.osd.ShowOSD", 0, name).Store(&r)
	if err != nil {
		return err
	}
	return nil
}
