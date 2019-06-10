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

package display

import (
	"fmt"

	"github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/api/drandr"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/strv"
)

const (
	dbusDest       = "com.deepin.daemon.Display"
	dbusPath       = "/com/deepin/daemon/Display"
	dbusIFC        = "com.deepin.daemon.Display"
	monitorDBusIFC = "com.deepin.daemon.Display.Monitor"
)

func (dpy *Manager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       dbusDest,
		ObjectPath: dbusPath,
		Interface:  dbusIFC,
	}
}

func (dpy *Manager) setPropHasChanged(v bool) {
	if dpy.HasChanged == v {
		return
	}
	dpy.HasChanged = v
	dbus.NotifyChange(dpy, "HasChanged")
}

func (dpy *Manager) setPropDisplayMode(v uint8) {
	if dpy.DisplayMode == v {
		return
	}
	dpy.DisplayMode = v
	dpy.setting.SetEnum(gsKeyDisplayMode, int32(v))
	dbus.NotifyChange(dpy, "DisplayMode")
}

func (dpy *Manager) setPropScreenSize(w, h uint16) {
	if dpy.ScreenWidth != w {
		dpy.ScreenWidth = w
		dbus.NotifyChange(dpy, "ScreenWidth")
	}
	if dpy.ScreenHeight != h {
		dpy.ScreenHeight = h
		dbus.NotifyChange(dpy, "ScreenHeight")
	}
}

func (dpy *Manager) setPropPrimary(v string) {
	if dpy.Primary == v {
		return
	}
	dpy.Primary = v
	dbus.NotifyChange(dpy, "Primary")
}

func (dpy *Manager) setPropCurrentCustomId(id string) {
	if dpy.CurrentCustomId == id {
		return
	}
	dpy.CurrentCustomId = id
	dbus.NotifyChange(dpy, "CurrentCustomId")
}

func (dpy *Manager) setPropCustomIdList(list []string) {
	if strv.Strv(dpy.CustomIdList).Equal(strv.Strv(list)) {
		return
	}
	dpy.CustomIdList = list
	dbus.NotifyChange(dpy, "CustomIdList")
}

func (dpy *Manager) setPropPrimaryRect(v x.Rectangle) {
	if dpy.PrimaryRect.X != v.X || dpy.PrimaryRect.Y != v.Y ||
		dpy.PrimaryRect.Width != v.Width || dpy.PrimaryRect.Height != v.Height {
		dpy.PrimaryRect = v
		dbus.NotifyChange(dpy, "PrimaryRect")
	}
}

func (dpy *Manager) setPropMonitors(v MonitorInfos) {
	// TODO: compare
	dpy.Monitors = v
	dbus.NotifyChange(dpy, "Monitors")
}

func (dpy *Manager) notifyBrightnessChange() {
	dbus.NotifyChange(dpy, "Brightness")
}

func (dpy *Manager) setPropTouchMap(v map[string]string) {
	dpy.TouchMap = v
	dbus.NotifyChange(dpy, "TouchMap")
}

func (m *MonitorInfo) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       dbusDest,
		ObjectPath: fmt.Sprintf("%s/Monitor%d", dbusPath, m.ID),
		Interface:  monitorDBusIFC,
	}
}

func (m *MonitorInfo) setPropEnabled(v bool) {
	if m.Enabled == v {
		return
	}
	m.Enabled = v
	dbus.NotifyChange(m, "Enabled")
}

func (m *MonitorInfo) setPropConnected(v bool) {
	if m.Connected == v {
		return
	}
	m.Connected = v
	dbus.NotifyChange(m, "Connected")
}

func (m *MonitorInfo) setPropX(v int16) {
	if m.X == v {
		return
	}
	m.X = v
	dbus.NotifyChange(m, "X")
}

func (m *MonitorInfo) setPropY(v int16) {
	if m.Y == v {
		return
	}
	m.Y = v
	dbus.NotifyChange(m, "Y")
}

func (m *MonitorInfo) setPropWidth(v uint16) {
	if m.Width == v {
		return
	}
	m.Width = v
	dbus.NotifyChange(m, "Width")
}

func (m *MonitorInfo) setPropHeight(v uint16) {
	if m.Height == v {
		return
	}
	m.Height = v
	dbus.NotifyChange(m, "Height")
}

func (m *MonitorInfo) setPropRefreshRate(v float64) {
	if m.RefreshRate == v {
		return
	}
	m.RefreshRate = v
	dbus.NotifyChange(m, "RefreshRate")
}

func (m *MonitorInfo) setPropRotation(v uint16) {
	if m.Rotation == v {
		return
	}
	m.Rotation = v
	dbus.NotifyChange(m, "Rotation")
}

func (m *MonitorInfo) setPropReflect(v uint16) {
	if m.Reflect == v {
		return
	}
	m.Reflect = v
	dbus.NotifyChange(m, "Reflect")
}

func (m *MonitorInfo) setPropCurrentMode(mode drandr.ModeInfo) {
	if m.CurrentMode.Equal(mode) {
		return
	}
	m.CurrentMode = mode
	dbus.NotifyChange(m, "CurrentMode")
}

func (m *MonitorInfo) setPropBestMode(mode drandr.ModeInfo) {
	if m.BestMode.Equal(mode) {
		return
	}
	m.BestMode = mode
	dbus.NotifyChange(m, "BestMode")
}

func (m *MonitorInfo) setPropRotations(v []uint16) {
	if uint16Splice(m.Rotations).equal(uint16Splice(v)) {
		return
	}
	m.Rotations = v
	dbus.NotifyChange(m, "Rotations")
}

func (m *MonitorInfo) setPropReflects(v []uint16) {
	if uint16Splice(m.Reflects).equal(uint16Splice(v)) {
		return
	}
	m.Reflects = v
	dbus.NotifyChange(m, "Reflects")
}

func (m *MonitorInfo) setPropModes(v drandr.ModeInfos) {
	if m.Modes.Equal(v) {
		return
	}
	m.Modes = v
	dbus.NotifyChange(m, "Modes")
}

func (m *MonitorInfo) setPropPreferredModes(v drandr.ModeInfos) {
	if m.PreferredModes.Equal(v) {
		return
	}
	m.PreferredModes = v
	dbus.NotifyChange(m, "PreferredModes")
}
