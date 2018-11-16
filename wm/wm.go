/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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
	"pkg.deepin.io/lib/log"
)

var _s *Switcher

// Start launch wm
func Start(logger *log.Logger, wmChooserLaunched bool) error {
	if _s != nil {
		return nil
	}

	_s = new(Switcher)
	_s.wmChooserLaunched = wmChooserLaunched
	_s.logger = logger
	_s.init()
	_s.listenStartupReady()
	_s.listenWMChanged()
	_s.adjustSogouSkin()

	err := dbus.InstallOnSession(_s)
	if err != nil {
		return err
	}
	dbus.DealWithUnhandledMessage()
	return nil
}

// GetWM return current window manager
func GetWM() string {
	if _s != nil {
		return _s.getWM()
	}
	return ""
}

func ShouldWait() bool {
	if _s != nil {
		return _s.shouldWait()
	}
	return true
}
