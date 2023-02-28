// SPDX-FileCopyrightText: 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("daemon/wl_display")

const (
	dbusServiceName = "org.deepin.dde.Display1"
	dbusInterface   = "org.deepin.dde.Display1"
	dbusPath        = "/org/deepin/dde/Display1"
)

var _dpy *Manager

func Start() error {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	service := dbusutil.NewService(sessionBus)
	m := newManager(service)
	m.init()
	err = service.Export(dbusPath, m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	_dpy = m
	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}

func GetRecommendedScaleFactor() float64 {
	if _dpy == nil {
		return 1.0
	}
	return _dpy.recommendScaleFactor
}
