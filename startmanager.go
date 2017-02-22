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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/howeyc/fsnotify"

	"dbus/com/deepin/daemon/apps"
	"gir/gio-2.0"
	"gir/glib-2.0"
	"github.com/BurntSushi/xgbutil"
	"pkg.deepin.io/lib/appinfo"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
	"pkg.deepin.io/lib/dbus"
)

const (
	_OBJECT = "com.deepin.SessionManager"
	_PATH   = "/com/deepin/StartManager"
	_INTER  = "com.deepin.StartManager"

	_WritePerm os.FileMode = 0200
)

const (
	_AUTOSTART             = "autostart"
	DESKTOP_ENV            = "Deepin"
	HiddenKey              = "Hidden"
	OnlyShowInKey          = "OnlyShowIn"
	NotShowInKey           = "NotShowIn"
	TryExecKey             = "TryExec"
	GnomeDelayKey          = "X-GNOME-Autostart-Delay"
	DeepinAutostartExecKey = "X-Deepin-Autostart-Exec"
)

type StartManager struct {
	userAutostartPath string
	AutostartChanged  func(string, string)
	delayHandler      *mapDelayHandler
	launchedRecorder  *apps.LaunchedRecorder
	launchContext     *appinfo.AppLaunchContext
}

func newStartManager(xu *xgbutil.XUtil) *StartManager {
	manager := &StartManager{}

	manager.launchContext = appinfo.NewAppLaunchContext(xu)
	manager.delayHandler = newMapDelayHandler(100*time.Millisecond,
		manager.emitSignalAutostartChanged)
	var err error
	manager.launchedRecorder, err = apps.NewLaunchedRecorder("com.deepin.daemon.Apps", "/com/deepin/daemon/Apps")
	if err != nil {
		logger.Warning("NewLaunchedRecorder failed:", err)
	}
	return manager
}

var START_MANAGER *StartManager

func (m *StartManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{_OBJECT, _PATH, _INTER}
}

func (m *StartManager) Launch(name string) (bool, error) {
	return m.LaunchWithTimestamp(name, 0)
}

func (m *StartManager) LaunchWithTimestamp(name string, timestamp uint32) (bool, error) {
	err := launch(name, timestamp, m.launchContext)
	if err != nil {
		logger.Warning("launch failed:", err)
	}

	// mark app launched
	if m.launchedRecorder != nil {
		m.launchedRecorder.MarkLaunched(name)
	}
	return err == nil, err
}

func launch(file string, timestamp uint32, ctx *appinfo.AppLaunchContext) error {
	appInfo, err := desktopappinfo.NewDesktopAppInfoFromFile(file)
	if err != nil {
		return err
	}
	ctx.SetTimestamp(timestamp)
	return appInfo.Launch(nil, ctx)
}

const (
	AutostartAdded         = "added"
	AutostartDeleted       = "deleted"
	SignalAutostartChanged = "AutostartChanged"
)

func (m *StartManager) emitSignalAutostartChanged(name string) {
	var status string
	if m.isAutostart(name) {
		status = AutostartAdded
	} else {
		status = AutostartDeleted
	}
	logger.Debugf("emit %v %q %q", SignalAutostartChanged, status, name)
	dbus.Emit(m, SignalAutostartChanged, status, name)
}

func (m *StartManager) listenAutostartFileEvents() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, dir := range m.autostartDirs() {
		logger.Debugf("Watch dir %q", dir)
		err := watcher.Watch(dir)
		if err != nil {
			logger.Warning(err)
		}
	}
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				name := path.Clean(ev.Name)
				basename := path.Base(name)
				matched, err := path.Match(`[^#.]*.desktop`, basename)
				if err != nil {
					logger.Warning(err)
				}
				if matched {
					logger.Debug("file event:", ev)
					m.delayHandler.AddTask(name)
				}

			case err := <-watcher.Error:
				logger.Error("fsnotify error:", err)
				return
			}
		}
	}()
}

// filepath.Walk will walk through the whole directory tree
func scanDir(d string, fn func(path string, info os.FileInfo) bool) {
	f, err := os.Open(d)
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	if err != nil {
		logger.Error("scanDir open dir failed:", err)
		return
	}
	// get all file info
	infos, err := f.Readdir(0)
	if err != nil {
		logger.Warning("scanDir Readdir error:", err)
	}

	for _, info := range infos {
		if fn(d, info) {
			break
		}
	}
}

func (m *StartManager) getUserAutostart(name string) string {
	return path.Join(m.getUserAutostartDir(), path.Base(name))
}

func (m *StartManager) isUserAutostart(name string) bool {
	if path.IsAbs(name) {
		if Exist(name) {
			return path.Dir(name) == m.getUserAutostartDir()
		}
		return false
	} else {
		return Exist(path.Join(m.getUserAutostartDir(), name))
	}
}

func showInDeepinAux(file *gio.DesktopAppInfo, keyname string) bool {
	s := file.GetString(keyname)
	if s == "" {
		return false
	}

	for _, env := range strings.Split(s, ";") {
		if strings.ToLower(env) == strings.ToLower(DESKTOP_ENV) {
			return true
		}
	}

	return false
}

func (m *StartManager) showInDeepin(file *gio.DesktopAppInfo) bool {
	if file.HasKey(NotShowInKey) {
		return !showInDeepinAux(file, NotShowInKey)
	} else if file.HasKey(OnlyShowInKey) {
		return showInDeepinAux(file, OnlyShowInKey)
	}

	return true
}

func findExec(_path, cmd string, exist chan<- bool) {
	found := false

	scanDir(_path, func(p string, info os.FileInfo) bool {
		if !info.IsDir() && info.Name() == cmd {
			found = true
			return true
		}
		return false
	})

	exist <- found
	return
}

func (m *StartManager) hasValidTryExecKey(file *gio.DesktopAppInfo) bool {
	// name := file.GetFilename()
	if !file.HasKey(TryExecKey) {
		// logger.Info(name, "No TryExec Key")
		return true
	}

	cmd := file.GetString(TryExecKey)
	if cmd == "" {
		// logger.Info(name, "TryExecKey is empty")
		return true
	}

	if path.IsAbs(cmd) {
		// logger.Info(cmd, "is exist?", Exist(cmd))
		if !Exist(cmd) {
			return false
		}

		stat, err := os.Lstat(cmd)
		if err != nil {
			return false
		}

		return (stat.Mode().Perm() & 0111) != 0
	} else {
		paths := strings.Split(os.Getenv("PATH"), ":")
		exist := make(chan bool)
		for _, _path := range paths {
			go findExec(_path, cmd, exist)
		}

		for _ = range paths {
			if t := <-exist; t {
				return true
			}
		}

		return false
	}
}

func (m *StartManager) isAutostartAux(name string) bool {
	file := gio.NewDesktopAppInfoFromFilename(name)
	if file == nil {
		return false
	}
	defer file.Unref()

	return m.hasValidTryExecKey(file) && !file.GetIsHidden() && m.showInDeepin(file)
}

func lowerBaseName(name string) string {
	return strings.ToLower(path.Base(name))
}

func (m *StartManager) isSystemStart(name string) bool {
	if path.IsAbs(name) {
		if !Exist(name) {
			return false
		}
		d := path.Dir(name)
		for i, dir := range m.autostartDirs() {
			if i == 0 {
				continue
			}
			if d == dir {
				return true
			}
		}
		return false
	} else {
		return Exist(m.getSysAutostart(name))
	}

}
func (m *StartManager) getSysAutostart(name string) string {
	sysPath := ""
	for i, d := range m.autostartDirs() {
		if i == 0 {
			continue
		}
		scanDir(d,
			func(p string, info os.FileInfo) bool {
				if lowerBaseName(name) == strings.ToLower(info.Name()) {
					sysPath = path.Join(p,
						info.Name())
					return true
				}
				return false
			},
		)
		if sysPath != "" {
			return sysPath
		}
	}
	return sysPath
}

func (m *StartManager) isAutostart(name string) bool {
	if !strings.HasSuffix(name, ".desktop") {
		return false
	}

	u := m.getUserAutostart(name)
	if Exist(u) {
		name = u
	} else {
		s := m.getSysAutostart(name)
		if s == "" {
			return false
		}
		name = s
	}

	return m.isAutostartAux(name)
}

func (m *StartManager) getAutostartApps(dir string) []string {
	apps := make([]string, 0)

	scanDir(dir, func(p string, info os.FileInfo) bool {
		if !info.IsDir() {
			fullpath := path.Join(p, info.Name())
			if m.isAutostart(fullpath) {
				apps = append(apps, fullpath)
			}
		}
		return false
	})

	return apps
}

func (m *StartManager) getUserAutostartDir() string {
	if m.userAutostartPath == "" {
		configPath := glib.GetUserConfigDir()
		m.userAutostartPath = path.Join(configPath, _AUTOSTART)
	}

	if !Exist(m.userAutostartPath) {
		err := os.MkdirAll(m.userAutostartPath, 0775)
		if err != nil {
			logger.Info(fmt.Errorf("create user autostart dir failed: %s", err))
		}
	}

	return m.userAutostartPath
}

func (m *StartManager) autostartDirs() []string {
	// first is user dir.
	dirs := []string{
		m.getUserAutostartDir(),
	}

	for _, configPath := range glib.GetSystemConfigDirs() {
		_path := path.Join(configPath, _AUTOSTART)
		if Exist(_path) {
			dirs = append(dirs, _path)
		}
	}

	return dirs
}

func (m *StartManager) AutostartList() []string {
	apps := make([]string, 0)
	dirs := m.autostartDirs()
	for _, dir := range dirs {
		if Exist(dir) {
			list := m.getAutostartApps(dir)
			if len(apps) == 0 {
				apps = append(apps, list...)
				continue
			}

			for _, v := range list {
				if isAppInList(v, apps) {
					continue
				}
				apps = append(apps, v)
			}
		}
	}
	return apps
}

func (m *StartManager) doSetAutostart(name string, autostart bool) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	if int(stat.Mode().Perm()&_WritePerm) == 0 {
		err := os.Chmod(name, stat.Mode()|_WritePerm)
		if err != nil {
			return err
		}
	}

	file := glib.NewKeyFile()
	defer file.Free()
	if ok, err := file.LoadFromFile(name, glib.KeyFileFlagsNone); !ok {
		return err
	}

	file.SetBoolean(
		glib.KeyFileDesktopGroup,
		HiddenKey,
		!autostart,
	)
	logger.Info("set autostart to", autostart)

	return saveKeyFile(file, name)
}

func (m *StartManager) addAutostartFile(name string) (string, error) {
	dst := m.getUserAutostart(name)
	// logger.Info(dst)
	if !Exist(dst) {
		src := m.getSysAutostart(name)
		if src == "" {
			src = name
		}

		err := copyFile(src, dst, CopyFileNotKeepSymlink)
		if err != nil {
			return dst, fmt.Errorf("copy file failed: %s", err)
		}

		k := glib.NewKeyFile()
		defer k.Free()

		k.LoadFromFile(dst, glib.KeyFileFlagsNone)
		exec, _ := k.GetString(glib.KeyFileDesktopGroup, DeepinAutostartExecKey)
		if exec != "" {
			k.SetString(glib.KeyFileDesktopGroup, glib.KeyFileDesktopKeyExec, exec)
		}
		saveKeyFile(k, dst)
	}

	return dst, nil
}

func (m *StartManager) setAutostart(name string, autostart bool) error {
	if !path.IsAbs(name) {
		file := gio.NewDesktopAppInfo(name)
		if file == nil {
			return errors.New("cannot create desktop file")
		}
		name = file.GetFilename()
		file.Unref()
	}
	// logger.Info(name, "autostart:", m.isAutostart(name))
	if autostart == m.isAutostart(name) {
		logger.Info("is already done")
		return nil
	}

	dst := name
	if !m.isUserAutostart(name) {
		// logger.Info("not user's")
		var err error
		dst, err = m.addAutostartFile(name)
		if err != nil {
			return err
		}
	}

	return m.doSetAutostart(dst, autostart)
}

func (m *StartManager) AddAutostart(name string) (bool, error) {
	err := m.setAutostart(name, true)
	if err != nil {
		logger.Info("AddAutostart", err)
		return false, err
	}
	return true, nil
}

func (m *StartManager) RemoveAutostart(name string) (bool, error) {
	err := m.setAutostart(name, false)
	if err != nil {
		logger.Info("RemoveAutostart failed:", err)
		return false, err
	}
	return true, nil
}

func (m *StartManager) IsAutostart(name string) bool {
	if !path.IsAbs(name) {
		file := gio.NewDesktopAppInfo(name)
		if file == nil {
			logger.Info(name, "is not a vaild desktop file.")
			return false
		}
		name = file.GetFilename()
		file.Unref()
	}
	return m.isAutostart(name)
}

func startStartManager(xu *xgbutil.XUtil) {
	gio.DesktopAppInfoSetDesktopEnv(DESKTOP_ENV)
	START_MANAGER = newStartManager(xu)
	if err := dbus.InstallOnSession(START_MANAGER); err != nil {
		logger.Error("Install StartManager Failed:", err)
	}
}

func startAutostartProgram() {
	START_MANAGER.listenAutostartFileEvents()
	// may be start N programs, like 5, at the same time is better than starting all programs at the same time.
	for _, path := range START_MANAGER.AutostartList() {
		go func(path string) {
			if delayTime := getDelayTime(path); delayTime != 0 {
				time.Sleep(delayTime)
			}

			START_MANAGER.Launch(path)
		}(path)
	}
}

func isAppInList(app string, apps []string) bool {
	for _, v := range apps {
		if path.Base(app) == path.Base(v) {
			return true
		}
	}
	return false
}
