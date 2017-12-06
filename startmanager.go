/*
 * Copyright (C) 2014 ~ 2017 Deepin Technology Co., Ltd.
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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dbus/com/deepin/daemon/apps"

	"gir/gio-2.0"
	"pkg.deepin.io/dde/startdde/swapsched"
	"pkg.deepin.io/lib/appinfo"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/fsnotify"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/strv"
	"pkg.deepin.io/lib/xdg/basedir"

	"github.com/BurntSushi/xgbutil"
)

const (
	startManagerObjPath   = "/com/deepin/StartManager"
	startManagerInterface = "com.deepin.StartManager"

	autostartDir      = "autostart"
	proxychainsBinary = "proxychains4"

	gSchemaLauncher        = "com.deepin.dde.launcher"
	gKeyAppsUseProxy       = "apps-use-proxy"
	gKeyAppsDisableScaling = "apps-disable-scaling"

	KeyXGnomeAutostartDelay = "X-GNOME-Autostart-Delay"
	KeyXDeepinCreatedBy     = "X-Deepin-CreatedBy"
	KeyXDeepinAppID         = "X-Deepin-AppID"
)

type StartManager struct {
	userAutostartPath   string
	AutostartChanged    func(string, string)
	delayHandler        *mapDelayHandler
	launchedRecorder    *apps.LaunchedRecorder
	launchContext       *appinfo.AppLaunchContext
	proxyChainsConfFile string
	proxyChainsBin      string

	appsDir            []string
	settings           *gio.Settings
	appsUseProxy       strv.Strv
	appsDisableScaling strv.Strv
	mu                 sync.Mutex
}

func newStartManager(xu *xgbutil.XUtil) *StartManager {
	m := &StartManager{}

	m.appsDir = getAppDirs()
	m.settings = gio.NewSettings(gSchemaLauncher)

	m.appsUseProxy = strv.Strv(m.settings.GetStrv(gKeyAppsUseProxy))
	m.appsDisableScaling = strv.Strv(m.settings.GetStrv(gKeyAppsDisableScaling))

	m.settings.Connect("changed", func(settings *gio.Settings, key string) {
		switch key {
		case gKeyAppsUseProxy:
			m.mu.Lock()
			m.appsUseProxy = strv.Strv(settings.GetStrv(key))
			m.mu.Unlock()
		case gKeyAppsDisableScaling:
			m.mu.Lock()
			m.appsDisableScaling = strv.Strv(settings.GetStrv(key))
			m.mu.Unlock()
		default:
			return
		}
		logger.Debug("update ", key)
	})

	m.proxyChainsConfFile = filepath.Join(basedir.GetUserConfigDir(), "deepin", "proxychains.conf")
	m.proxyChainsBin, _ = exec.LookPath(proxychainsBinary)
	logger.Debugf("startManager proxychain confFile %q, bin: %q", m.proxyChainsConfFile, m.proxyChainsBin)

	m.launchContext = appinfo.NewAppLaunchContext(xu)
	m.delayHandler = newMapDelayHandler(100*time.Millisecond,
		m.emitSignalAutostartChanged)
	var err error
	m.launchedRecorder, err = apps.NewLaunchedRecorder("com.deepin.daemon.Apps", "/com/deepin/daemon/Apps")
	if err != nil {
		logger.Warning("NewLaunchedRecorder failed:", err)
	}
	return m
}

var START_MANAGER *StartManager

func (m *StartManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       START_DDE_DEST,
		ObjectPath: startManagerObjPath,
		Interface:  startManagerInterface,
	}
}

func (m *StartManager) Launch(desktopFile string) (bool, error) {
	return m.LaunchWithTimestamp(desktopFile, 0)
}

func (m *StartManager) LaunchWithTimestamp(desktopFile string, timestamp uint32) (bool, error) {
	err := m.LaunchApp(desktopFile, timestamp, nil)
	return err == nil, err
}

func (m *StartManager) LaunchApp(desktopFile string, timestamp uint32, files []string) error {
	err := m.launchApp(desktopFile, timestamp, files, m.launchContext)
	if err != nil {
		logger.Warning("launch failed:", err)
	}

	// mark app launched
	if m.launchedRecorder != nil {
		m.launchedRecorder.MarkLaunched(desktopFile)
	}
	return err
}

func (m *StartManager) LaunchAppAction(desktopFile, action string, timestamp uint32) error {
	err := m.launchAppAction(desktopFile, action, timestamp, m.launchContext)
	if err != nil {
		logger.Warning("launch failed:", err)
	}
	// mark app launched
	if m.launchedRecorder != nil {
		m.launchedRecorder.MarkLaunched(desktopFile)
	}
	return err
}

func (m *StartManager) RunCommand(exe string, args []string) error {
	var uiApp *swapsched.UIApp
	var err error
	if swapSchedDispatcher != nil {
		uiApp, err = swapSchedDispatcher.NewApp(exe)
		if err != nil {
			logger.Warning("dispatcher.NewApp error:", err)
		}
	}

	var cmd *exec.Cmd
	if uiApp != nil {
		args = append([]string{"-g", "memory:" + uiApp.GetCGroup(), exe}, args...)
		cmd = exec.Command("cgexec", args...)
	} else {
		cmd = exec.Command(exe, args...)
	}

	err = cmd.Start()
	return waitCmd(cmd, err, uiApp)
}

func (m *StartManager) getAppIdByFilePath(file string) string {
	return getAppIdByFilePath(file, m.appsDir)
}

func (m *StartManager) shouldUseProxy(id string) bool {
	m.mu.Lock()
	if !m.appsUseProxy.Contains(id) {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()

	if _, err := os.Stat(m.proxyChainsConfFile); err != nil {
		return false
	}

	if m.proxyChainsBin == "" {
		// try get proxyChainsBin again
		m.proxyChainsBin, _ = exec.LookPath(proxychainsBinary)
		if m.proxyChainsBin == "" {
			return false
		}
	}
	return true
}

func (m *StartManager) shouldDisableScaling(id string) bool {
	m.mu.Lock()
	contains := m.appsDisableScaling.Contains(id)
	m.mu.Unlock()
	return contains
}

type IStartCommand interface {
	StartCommand(files []string, ctx *appinfo.AppLaunchContext) (*exec.Cmd, error)
}

func (m *StartManager) launch(appInfo *desktopappinfo.DesktopAppInfo, timestamp uint32,
	files []string, ctx *appinfo.AppLaunchContext, iStartCmd IStartCommand) error {

	desktopFile := appInfo.GetFileName()
	logger.Debug("launch: desktopFile is", desktopFile)
	var err error
	var cmdPrefixes []string
	var uiApp *swapsched.UIApp
	if swapSchedDispatcher != nil && !isDEComponent(appInfo) {
		uiApp, err = swapSchedDispatcher.NewApp(desktopFile)
		if err != nil {
			logger.Warning("dispatcher.NewApp error:", err)
		} else {
			logger.Debug("launch: use cgexec")
			cmdPrefixes = []string{"cgexec", "-g", "memory:" + uiApp.GetCGroup()}
		}
	}

	appId := m.getAppIdByFilePath(desktopFile)
	if appId != "" {
		if m.shouldUseProxy(appId) {
			logger.Debug("launch: use proxy")
			cmdPrefixes = append(cmdPrefixes, m.proxyChainsBin, "-f", m.proxyChainsConfFile)
		}
		if m.shouldDisableScaling(appId) {
			logger.Debug("launch: disable scaling")
			cmdPrefixes = append(cmdPrefixes, "/usr/bin/env", "GDK_SCALE=1",
				"QT_SCALE_FACTOR=1")
		}
	}

	logger.Debug("cmd prefiexs:", cmdPrefixes)
	ctx.Lock()
	ctx.SetTimestamp(timestamp)
	ctx.SetCmdPrefixes(cmdPrefixes)
	cmd, err := iStartCmd.StartCommand(files, ctx)
	ctx.Unlock()
	return waitCmd(cmd, err, uiApp)
}

func newDesktopAppInfoFromFile(filename string) (*desktopappinfo.DesktopAppInfo, error) {
	dai, err := desktopappinfo.NewDesktopAppInfoFromFile(filename)
	if err != nil {
		return nil, err
	}

	if !dai.IsInstalled() {
		createdBy, _ := dai.GetString(desktopappinfo.MainSection, KeyXDeepinCreatedBy)
		if createdBy != "" {
			appId, _ := dai.GetString(desktopappinfo.MainSection, KeyXDeepinAppID)
			dai1 := desktopappinfo.NewDesktopAppInfo(appId)
			if dai1 != nil {
				dai = dai1
			}
		}
	}
	return dai, nil
}

func (m *StartManager) launchApp(desktopFile string, timestamp uint32, files []string, ctx *appinfo.AppLaunchContext) error {
	appInfo, err := newDesktopAppInfoFromFile(desktopFile)
	if err != nil {
		return err
	}

	return m.launch(appInfo, timestamp, files, ctx, appInfo)
}

func (m *StartManager) launchAppAction(desktopFile, actionSection string, timestamp uint32, ctx *appinfo.AppLaunchContext) error {
	appInfo, err := newDesktopAppInfoFromFile(desktopFile)
	if err != nil {
		return err
	}

	var targetAction desktopappinfo.DesktopAction
	actions := appInfo.GetActions()
	for _, action := range actions {
		if action.Section == actionSection {
			targetAction = action
		}
	}

	if targetAction.Section == "" {
		return fmt.Errorf("not found section %q in %q", actionSection, desktopFile)
	}

	return m.launch(appInfo, timestamp, nil, ctx, &targetAction)
}

func waitCmd(cmd *exec.Cmd, err error, uiApp *swapsched.UIApp) error {
	if uiApp != nil {
		swapSchedDispatcher.AddApp(uiApp)
	}

	if err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning(err)
		}
		if uiApp != nil {
			uiApp.SetStateEnd()
		}
	}()
	return nil
}

func isDEComponent(appInfo *desktopappinfo.DesktopAppInfo) bool {
	isDEComponent, _ := appInfo.GetBool(desktopappinfo.MainSection, "X-Deepin-DEComponent")
	return isDEComponent
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
				name := filepath.Clean(ev.Name)
				basename := filepath.Base(name)
				matched, err := filepath.Match(`[^#.]*.desktop`, basename)
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
func scanDir(dir string, fn func(dir string, info os.FileInfo) bool) {
	f, err := os.Open(dir)
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
	fileInfoList, err := f.Readdir(0)
	if err != nil {
		logger.Warning("scanDir Readdir error:", err)
	}

	for _, info := range fileInfoList {
		if fn(dir, info) {
			break
		}
	}
}

func (m *StartManager) getUserAutostart(name string) string {
	return filepath.Join(m.getUserAutostartDir(), filepath.Base(name))
}

func (m *StartManager) isUserAutostart(name string) bool {
	if filepath.IsAbs(name) {
		if Exist(name) {
			return filepath.Dir(name) == m.getUserAutostartDir()
		}
		return false
	} else {
		return Exist(filepath.Join(m.getUserAutostartDir(), name))
	}
}

func (m *StartManager) isAutostartAux(filename string) bool {
	dai, err := desktopappinfo.NewDesktopAppInfoFromFile(filename)
	if err != nil {
		return false
	}

	// ignore key NoDisplay
	if dai.GetIsHiden() {
		return false
	}
	return dai.GetShowIn(nil)
}

func lowerBaseName(name string) string {
	return strings.ToLower(filepath.Base(name))
}

func (m *StartManager) isSystemStart(name string) bool {
	if filepath.IsAbs(name) {
		if !Exist(name) {
			return false
		}
		d := filepath.Dir(name)
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
	for idx, dir := range m.autostartDirs() {
		if idx == 0 {
			continue
		}
		scanDir(dir,
			func(dir0 string, fileInfo os.FileInfo) bool {
				if lowerBaseName(name) == strings.ToLower(fileInfo.Name()) {
					sysPath = filepath.Join(dir0, fileInfo.Name())
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

func (m *StartManager) isAutostart(filename string) bool {
	if !strings.HasSuffix(filename, ".desktop") {
		return false
	}

	u := m.getUserAutostart(filename)
	if Exist(u) {
		filename = u
	} else {
		s := m.getSysAutostart(filename)
		if s == "" {
			return false
		}
		filename = s
	}

	return m.isAutostartAux(filename)
}

func (m *StartManager) getAutostartApps(dir string) []string {
	apps := make([]string, 0)

	scanDir(dir, func(p string, info os.FileInfo) bool {
		if !info.IsDir() {
			fullpath := filepath.Join(p, info.Name())
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
		configPath := basedir.GetUserConfigDir()
		m.userAutostartPath = filepath.Join(configPath, autostartDir)
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

	for _, configPath := range basedir.GetSystemConfigDirs() {
		_path := filepath.Join(configPath, autostartDir)
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

func (m *StartManager) addAutostartFile(name string) (string, error) {
	dst := m.getUserAutostart(name)
	if !Exist(dst) {
		src := m.getSysAutostart(name)
		if src == "" {
			src = name
		}

		err := copyFile(src, dst, CopyFileNotKeepSymlink)
		if err != nil {
			return dst, fmt.Errorf("copy file failed: %s", err)
		}
	}

	return dst, nil
}

func (m *StartManager) setAutostart(filename string, val bool) error {
	appId := m.getAppIdByFilePath(filename)
	if appId == "" {
		return errors.New("failed to get app id")
	}

	if val == m.isAutostart(filename) {
		logger.Info("is already done")
		return nil
	}

	dst := filename
	if !m.isUserAutostart(filename) {
		// logger.Info("not user's")
		var err error
		dst, err = m.addAutostartFile(filename)
		if err != nil {
			return err
		}
	}

	return m.doSetAutostart(dst, appId, val)
}

func (m *StartManager) doSetAutostart(filename, appId string, autostart bool) error {
	keyFile := keyfile.NewKeyFile()
	if err := keyFile.LoadFromFile(filename); err != nil {
		return err
	}

	keyFile.SetString(desktopappinfo.MainSection, KeyXDeepinCreatedBy, START_DDE_DEST)
	keyFile.SetString(desktopappinfo.MainSection, KeyXDeepinAppID, appId)
	keyFile.SetBool(desktopappinfo.MainSection, desktopappinfo.KeyHidden, !autostart)
	logger.Info("set autostart to", autostart)
	return keyFile.SaveToFile(filename)
}

func (m *StartManager) AddAutostart(filename string) (bool, error) {
	err := m.setAutostart(filename, true)
	if err != nil {
		logger.Warning("AddAutostart failed:", err)
		return false, err
	}
	return true, nil
}

func (m *StartManager) RemoveAutostart(filename string) (bool, error) {
	err := m.setAutostart(filename, false)
	if err != nil {
		logger.Warning("RemoveAutostart failed:", err)
		return false, err
	}
	return true, nil
}

func (m *StartManager) IsAutostart(filename string) bool {
	return m.isAutostart(filename)
}

func startStartManager(xu *xgbutil.XUtil) {
	START_MANAGER = newStartManager(xu)
	if err := dbus.InstallOnSession(START_MANAGER); err != nil {
		logger.Error("Install StartManager Failed:", err)
	}
}

func startAutostartProgram() {
	START_MANAGER.listenAutostartFileEvents()
	// may be start N programs, like 5, at the same time is better than starting all programs at the same time.
	for _, desktopFile := range START_MANAGER.AutostartList() {
		go func(desktopFile string) {
			if delayTime := getDelayTime(desktopFile); delayTime != 0 {
				time.Sleep(delayTime)
			}

			START_MANAGER.LaunchApp(desktopFile, 0, nil)
		}(desktopFile)
	}
}

func isAppInList(app string, apps []string) bool {
	for _, v := range apps {
		if filepath.Base(app) == filepath.Base(v) {
			return true
		}
	}
	return false
}
