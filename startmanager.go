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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	dbus "github.com/godbus/dbus"
	daemonApps "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.apps"
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/startdde/swapsched"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/appinfo"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/strv"
	"pkg.deepin.io/lib/xdg/basedir"
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
	KeyXGnomeAutoRestart    = "X-GNOME-AutoRestart"
	KeyXDeepinCreatedBy     = "X-Deepin-CreatedBy"
	KeyXDeepinAppID         = "X-Deepin-AppID"

	uiAppSchedHooksDir = "/usr/lib/UIAppSched.hooks"
	launchedHookDir    = uiAppSchedHooksDir + "/launched"

	restartRateLimitSeconds = 60
)

type StartManager struct {
	service             *dbusutil.Service
	userAutostartPath   string
	delayHandler        *mapDelayHandler
	daemonApps          *daemonApps.Apps
	launchContext       *appinfo.AppLaunchContext
	restartTimeMap      map[string]time.Time
	restartTimeMapMu    sync.Mutex
	proxyChainsConfFile string
	proxyChainsBin      string
	appsDir             []string
	settings            *gio.Settings
	appsUseProxy        strv.Strv
	appsDisableScaling  strv.Strv
	mu                  sync.Mutex
	appClose            chan *UeMessageItem
	launchedHooks       []string

	NeededMemory uint64

	//nolint
	signals *struct {
		AutostartChanged struct {
			status string
			name   string
		}
	}

	//nolint
	methods *struct {
		IsMemSufficient       func() `out:"result"`
		TryAgain              func() `in:"launch"`
		DumpMemRecord         func() `out:"record"`
		GetApps               func() `out:"apps"`
		Launch                func() `in:"desktopFile" out:"ok"`
		LaunchWithTimestamp   func() `in:"desktopFile,timestamp" out:"ok"`
		LaunchApp             func() `in:"desktopFile,timestamp,files"`
		LaunchAppWithOptions  func() `in:"desktopFile,timestamp,files,options"`
		LaunchAppAction       func() `in:"desktopFile,action,timestamp"`
		RunCommand            func() `in:"exe,args"`
		RunCommandWithOptions func() `in:"exe,args,options"`
		AutostartList         func() `out:"list"`
		AddAutostart          func() `in:"filename" out:"ok"`
		RemoveAutostart       func() `in:"filename" out:"ok"`
		IsAutostart           func() `in:"filename" out:"result"`
	}
}

func getLaunchedHooks() (ret []string) {
	dir := filepath.Join(launchedHookDir)
	fileInfoList, err := ioutil.ReadDir(dir)
	if err != nil {
		logger.Warning(err)
		return
	}

	for _, fileInfo := range fileInfoList {
		if fileInfo.IsDir() {
			continue
		}
		logger.Debug("load launched hook", fileInfo.Name())
		ret = append(ret, fileInfo.Name())
	}
	return
}

func (m *StartManager) execLaunchedHooks(desktopFile, cGroupName string) {
	for _, name := range m.launchedHooks {
		p := filepath.Join(launchedHookDir, name)
		err := exec.Command(p, desktopFile, cGroupName).Run()
		if err != nil {
			logger.Warning("run cmd failed:", err)
		}
	}
}

func newStartManager(conn *x.Conn, service *dbusutil.Service) *StartManager {
	m := &StartManager{
		service: service,
	}

	m.appsDir = getAppDirs()
	m.settings = gio.NewSettings(gSchemaLauncher)
	m.appClose = make(chan *UeMessageItem, UserExperCLoseAppChanInitLen)

	m.appsUseProxy = m.settings.GetStrv(gKeyAppsUseProxy)
	m.appsDisableScaling = m.settings.GetStrv(gKeyAppsDisableScaling)

	gsettings.ConnectChanged(gSchemaLauncher, "*", func(key string) {
		switch key {
		case gKeyAppsUseProxy:
			m.mu.Lock()
			m.appsUseProxy = strv.Strv(m.settings.GetStrv(key))
			m.mu.Unlock()
		case gKeyAppsDisableScaling:
			m.mu.Lock()
			m.appsDisableScaling = strv.Strv(m.settings.GetStrv(key))
			m.mu.Unlock()
		default:
			return
		}
		logger.Debug("update ", key)
	})

	err := m.listenAppCloseEvent()
	if err != nil {
		logger.Warning(err)
	}

	m.proxyChainsConfFile = filepath.Join(basedir.GetUserConfigDir(), "deepin", "proxychains.conf")
	m.proxyChainsBin, _ = exec.LookPath(proxychainsBinary)
	logger.Debugf("startManager proxychain confFile %q, bin: %q", m.proxyChainsConfFile, m.proxyChainsBin)

	m.launchContext = appinfo.NewAppLaunchContext(conn)
	m.restartTimeMap = make(map[string]time.Time)
	m.launchedHooks = getLaunchedHooks()
	m.delayHandler = newMapDelayHandler(100*time.Millisecond,
		m.emitSignalAutostartChanged)
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
	}

	m.daemonApps = daemonApps.NewApps(sysBus)
	return m
}

var _startManager *StartManager

func (m *StartManager) GetInterfaceName() string {
	return startManagerInterface
}

func (m *StartManager) getRestartTime(appInfo *desktopappinfo.DesktopAppInfo) (time.Time, bool) {
	filename := appInfo.GetFileName()
	m.restartTimeMapMu.Lock()
	t, ok := m.restartTimeMap[filename]
	m.restartTimeMapMu.Unlock()
	return t, ok
}

func (m *StartManager) setRestartTime(appInfo *desktopappinfo.DesktopAppInfo, t time.Time) {
	filename := appInfo.GetFileName()
	m.restartTimeMapMu.Lock()
	m.restartTimeMap[filename] = t
	m.restartTimeMapMu.Unlock()
}

func (m *StartManager) GetApps() (map[uint32]string, *dbus.Error) {
	if swapSchedDispatcher == nil {
		return nil, dbusutil.ToError(errors.New("swap-sched disabled"))
	}

	return swapSchedDispatcher.GetAppsSeqDescMap(), nil
}

// deprecated
func (m *StartManager) Launch(sender dbus.Sender, desktopFile string) (bool, *dbus.Error) {
	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	err = m.launchAppWithOptions(desktopFile, 0, nil, nil)
	return err == nil, dbusutil.ToError(err)
}

// deprecated
func (m *StartManager) LaunchWithTimestamp(sender dbus.Sender, desktopFile string,
	timestamp uint32) (bool, *dbus.Error) {

	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	err = m.launchAppWithOptions(desktopFile, timestamp, nil, nil)
	return err == nil, dbusutil.ToError(err)
}

func (m *StartManager) LaunchApp(sender dbus.Sender, desktopFile string,
	timestamp uint32, files []string) *dbus.Error {

	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = m.launchAppWithOptions(desktopFile, timestamp, files, nil)
	return dbusutil.ToError(err)
}

func (m *StartManager) LaunchAppWithOptions(sender dbus.Sender, desktopFile string,
	timestamp uint32, files []string, options map[string]dbus.Variant) *dbus.Error {

	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = m.launchAppWithOptions(desktopFile, timestamp, files, options)
	return dbusutil.ToError(err)
}

func (m *StartManager) launchAppWithOptions(desktopFile string, timestamp uint32,
	files []string, options map[string]dbus.Variant) error {

	err := handleMemInsufficient(desktopFile)
	if err != nil {
		if getCurAction() != "" {
			return nil
		}
		_app.desktop = desktopFile
		_app.timestamp = timestamp
		_app.files = files
		_app.options = options
		setCurAction("LaunchApp")
		return nil
	}

	err = m.launchApp(desktopFile, timestamp, files, m.launchContext, options)
	if err != nil {
		logger.Warning("launch failed:", err)
	}

	// mark app launched
	if m.daemonApps != nil {
		err := m.daemonApps.MarkLaunched(0, desktopFile)
		if err != nil {
			logger.Warning(err)
		}
	}
	return err
}

func (m *StartManager) LaunchAppAction(sender dbus.Sender, desktopFile, action string,
	timestamp uint32) *dbus.Error {

	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = m.launchAppAction(desktopFile, action, timestamp)
	return dbusutil.ToError(err)
}

func (m *StartManager) launchAppAction(desktopFile, action string, timestamp uint32) error {
	err := handleMemInsufficient(desktopFile + action)
	if err != nil {
		if getCurAction() != "" {
			return nil
		}
		_appAction.desktop = desktopFile
		_appAction.action = action
		_appAction.timestamp = timestamp
		setCurAction("LaunchAppAction")
		return nil
	}

	err = m.launchAppActionAux(desktopFile, action, timestamp, m.launchContext)
	if err != nil {
		logger.Warning("launch failed:", err)
	}
	// mark app launched
	if m.daemonApps != nil {
		err := m.daemonApps.MarkLaunched(0, desktopFile)
		if err != nil {
			logger.Warning(err)
		}
	}
	return err
}

func getCmdDesc(exe string, args []string) string {
	const prefix = "cmd:"
	if (exe == "sh" || exe == "/bin/sh") &&
		len(args) == 2 && args[0] == "-c" {
		// sh -c cmdline
		// or /bin/sh -c cmdline
		return prefix + args[1]
	}
	if len(args) > 0 {
		return prefix + exe + " " + strings.Join(args, " ")
	}
	return prefix + exe
}

func (m *StartManager) RunCommand(sender dbus.Sender, exe string, args []string) *dbus.Error {
	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = m.runCommandWithOptions(exe, args, nil)
	return dbusutil.ToError(err)
}

func (m *StartManager) RunCommandWithOptions(sender dbus.Sender, exe string, args []string,
	options map[string]dbus.Variant) *dbus.Error {

	err := checkDMsgUid(m.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = m.runCommandWithOptions(exe, args, options)
	return dbusutil.ToError(err)
}

func checkDMsgUid(service *dbusutil.Service, sender dbus.Sender) error {
	uid, err := service.GetConnUID(string(sender))
	if err != nil {
		return err
	}
	if os.Getuid() == int(uid) {
		return nil
	}
	return errors.New("permission denied")
}

func (m *StartManager) runCommandWithOptions(exe string, args []string,
	options map[string]dbus.Variant) error {

	var _name = exe
	if len(args) != 0 {
		_name += " " + strings.Join(args, " ")
	}
	err := handleMemInsufficient(_name)
	if err != nil {
		if getCurAction() != "" {
			return nil
		}
		_cmd.exe = exe
		_cmd.args = args
		_cmd.options = options
		setCurAction("RunCommand")
		return nil
	}

	var uiApp *swapsched.UIApp
	if swapSchedDispatcher != nil {
		desc := getCmdDesc(exe, args)
		uiApp, err = swapSchedDispatcher.NewApp(desc, nil)
		if err != nil {
			logger.Warning("dispatcher.NewApp error:", err)
		}
	}

	var cmd *exec.Cmd
	if uiApp != nil {
		args = append([]string{"-g", "memory:" + uiApp.GetCGroup(), exe}, args...)
		cmd = exec.Command(globalCgExecBin, args...)
	} else {
		cmd = exec.Command(exe, args...)
	}

	if dirVar, ok := options["dir"]; ok {
		if dirStr, ok := dirVar.Value().(string); ok {
			cmd.Dir = dirStr
		} else {
			return errors.New("type of option dir is not string")
		}
	}

	err = cmd.Start()
	return m.waitCmd(nil, cmd, err, uiApp, _name)
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
	files []string, ctx *appinfo.AppLaunchContext, iStartCmd IStartCommand, cmdName string) error {

	// maximum RAM unit is MB
	maxRAM, _ := appInfo.GetUint64(desktopappinfo.MainSection, "X-Deepin-MaximumRAM")
	// unit is MB/s
	blkioReadMBPS, _ := appInfo.GetUint64(desktopappinfo.MainSection, "X-Deepin-BlkioReadMBPS")
	blkioWriteMBPS, _ := appInfo.GetUint64(desktopappinfo.MainSection, "X-Deepin-BlkioWriteMBPS")

	desktopFile := appInfo.GetFileName()
	logger.Debug("launch: desktopFile is", desktopFile)
	var err error
	var cmdPrefixes []string
	var cmdSuffixes []string
	var uiApp *swapsched.UIApp
	if swapSchedDispatcher != nil {
		if isDEComponent(appInfo) {
			cmdPrefixes = []string{globalCgExecBin, "-g", "memory:" + swapSchedDispatcher.GetDECGroup()}
		} else {
			limit := &swapsched.AppResourcesLimit{
				MemHardLimit:  maxRAM * 1e6,
				BlkioReadBPS:  blkioReadMBPS * 1e6,
				BlkioWriteBPS: blkioWriteMBPS * 1e6,
			}
			logger.Debugf("launch limit: %#v", limit)
			uiApp, err = swapSchedDispatcher.NewApp(desktopFile, limit)
			if err != nil {
				logger.Warning("dispatcher.NewApp error:", err)
			} else {
				logger.Debug("launch: use cgexec")
				cmdPrefixes = []string{globalCgExecBin, "-g",
					"memory,freezer,blkio:" + uiApp.GetCGroup()}
			}
		}
	}

	appId := m.getAppIdByFilePath(desktopFile)
	if appId != "" {
		if m.shouldUseProxy(appId) {
			logger.Debug("launch: use proxy")
			if supportProxyServerOption(appId) {
				proxyServerUrl, err := getProxyServerUrl()
				if err == nil {
					cmdSuffixes = append(cmdSuffixes, "--proxy-server="+proxyServerUrl)
				} else {
					logger.Warning("failed to get google chrome proxy server url:", err)
				}
			} else {
				cmdPrefixes = append(cmdPrefixes, m.proxyChainsBin, "-f", m.proxyChainsConfFile)
			}
		}
		if m.shouldDisableScaling(appId) {
			logger.Debug("launch: disable scaling")
			gs := gio.NewSettings("com.deepin.xsettings")
			defer gs.Unref()
			scale := gs.GetDouble("scale-factor")
			if scale > 0 {
				scale = 1 / scale
			} else {
				scale = 1
			}
			qt := "QT_SCALE_FACTOR=" + strconv.FormatFloat(scale, 'f', -1, 64)
			cmdPrefixes = append(cmdPrefixes, "/usr/bin/env", "GDK_DPI_SCALE=1", "GDK_SCALE=1", qt)
		}
	}

	logger.Debug("cmd prefiexs:", cmdPrefixes)
	ctx.Lock()
	ctx.SetTimestamp(timestamp)
	ctx.SetCmdPrefixes(cmdPrefixes)
	ctx.SetCmdSuffixes(cmdSuffixes)
	cmd, err := iStartCmd.StartCommand(files, ctx)
	ctx.Unlock()

	// exec launched hooks
	cGroupName := ""
	if uiApp != nil {
		cGroupName = uiApp.GetCGroup()
	}
	go m.execLaunchedHooks(desktopFile, cGroupName)

	item := &UeMessageItem{Path: appInfo.GetFileName(), Name: appInfo.GetName(), Id: appInfo.GetId()}
	sendAppDataMsgToUserExperModule(UserExperOpenApp, item)

	return m.waitCmd(appInfo, cmd, err, uiApp, cmdName)
}

func (m *StartManager) listenAppCloseEvent() error {
	go func() {
		for item := range m.appClose {
			if item != nil {
				item := &UeMessageItem{Path: item.Path, Name: item.Name, Id: item.Id}
				sendAppDataMsgToUserExperModule(UserExperCloseApp, item)
			}
		}
	}()

	return nil
}

func sendAppDataMsgToUserExperModule(msg string, item *UeMessageItem) {
	// send open app and close app message to user experience module
	bus, err := dbus.SystemBus()
	if err == nil {
		userexp := bus.Object(UserExperServiceName, UserExperPath)
		err = userexp.Call(UserExperServiceName+".SendAppStateData", 0, msg, item.Path, item.Name, item.Id).Err
		if err != nil {
			logger.Warningf("failed to call %s.SendAppStateData, %v", UserExperServiceName, err)
		} else {
			logger.Infof("send %s message, app info %s/%s/%s", msg, item.Path, item.Name, item.Id)
		}
	} else {
		logger.Warning(err)
	}
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

func (m *StartManager) launchApp(desktopFile string, timestamp uint32, files []string, ctx *appinfo.AppLaunchContext, options map[string]dbus.Variant) error {
	appInfo, err := newDesktopAppInfoFromFile(desktopFile)
	if err != nil {
		return err
	}

	if pathVar, ok := options["path"]; ok {
		pathStr, isStr := pathVar.Value().(string)
		if !isStr {
			return errors.New("type of option path is not string")
		}
		appInfo.SetString(desktopappinfo.MainSection, desktopappinfo.KeyPath, pathStr)
	}

	return m.launch(appInfo, timestamp, files, ctx, appInfo, desktopFile)
}

func (m *StartManager) launchAppActionAux(desktopFile, actionSection string, timestamp uint32, ctx *appinfo.AppLaunchContext) error {
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

	return m.launch(appInfo, timestamp, nil, ctx, &targetAction, desktopFile+actionSection)
}

func (m *StartManager) waitCmd(appInfo *desktopappinfo.DesktopAppInfo, cmd *exec.Cmd, err error,
	uiApp *swapsched.UIApp, cmdName string) error {
	if uiApp != nil {
		swapSchedDispatcher.AddApp(uiApp)
	}

	if err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()

		// send app close info to ue module
		// we did not care the program exit normal or not
		if appInfo != nil {
			item := &UeMessageItem{Path: appInfo.GetFileName(), Name: appInfo.GetName(), Id: appInfo.GetId()}
			m.appClose <- item
		}

		if err != nil {
			logger.Warningf("%v: %v", cmd.Args, err)

			if appInfo != nil {
				autoRestart, _ := appInfo.GetBool(desktopappinfo.MainSection, KeyXGnomeAutoRestart)
				if autoRestart {
					now := time.Now()

					canLaunch := true
					if lastRestartTime, ok := m.getRestartTime(appInfo); ok {
						elapsed := now.Sub(lastRestartTime)
						if elapsed < restartRateLimitSeconds*time.Second {
							logger.Warningf("app %q re-spawning too quickly", appInfo.GetFileName())
							canLaunch = false
						}
					}

					if canLaunch {
						err = m.launch(appInfo, 0, nil, m.launchContext, appInfo,
							appInfo.GetFileName())
						if err != nil {
							logger.Warningf("failed to restart app %q", appInfo.GetFileName())
						}
						m.setRestartTime(appInfo, now)
					}
				}
			}
		}
		if uiApp != nil {
			uiApp.SetStateEnd()
		}
	}()
	if uiApp != nil {
		go func() {
			err := saveNeededMemory(cmdName, uiApp.GetCGroup())
			if err != nil {
				logger.Warning(err)
			}
		}()
	}

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
	err := m.service.Emit(m, SignalAutostartChanged, status, name)
	if err != nil {
		logger.Warning("failed to emit signal AutostartChanged:", err)
	}
}

func (m *StartManager) listenAutostartFileEvents() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, dir := range m.autostartDirs() {
		logger.Debugf("Watch dir %q", dir)
		err := watcher.Add(dir)
		if err != nil {
			logger.Warning(err)
		}
	}
	go func() {
		for {
			select {
			case ev, ok := <-watcher.Events:
				if !ok {
					logger.Error("Invalid watcher event:", ev)
					return
				}

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

			case err := <-watcher.Errors:
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

func (m *StartManager) AutostartList() ([]string, *dbus.Error) {
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
	return apps, nil
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

	keyFile.SetString(desktopappinfo.MainSection, KeyXDeepinCreatedBy, sessionManagerServiceName)
	keyFile.SetString(desktopappinfo.MainSection, KeyXDeepinAppID, appId)
	keyFile.SetBool(desktopappinfo.MainSection, desktopappinfo.KeyHidden, !autostart)
	logger.Info("set autostart to", autostart)
	return keyFile.SaveToFile(filename)
}

func (m *StartManager) AddAutostart(filename string) (bool, *dbus.Error) {
	err := m.setAutostart(filename, true)
	if err != nil {
		logger.Warning("AddAutostart failed:", err)
		return false, dbusutil.ToError(err)
	}
	return true, nil
}

func (m *StartManager) RemoveAutostart(filename string) (bool, *dbus.Error) {
	err := m.setAutostart(filename, false)
	if err != nil {
		logger.Warning("RemoveAutostart failed:", err)
		return false, dbusutil.ToError(err)
	}
	return true, nil
}

func (m *StartManager) IsAutostart(filename string) (bool, *dbus.Error) {
	return m.isAutostart(filename), nil
}

func startStartManager(conn *x.Conn, service *dbusutil.Service) {
	_startManager = newStartManager(conn, service)
	err := service.Export(startManagerObjPath, _startManager)
	if err != nil {
		logger.Warning("export StartManager failed:", err)
	}
}

func startAutostartProgram() {
	// may be start N programs, like 5, at the same time is better than starting all programs at the same time.
	autoStartList, _ := _startManager.AutostartList()
	for _, desktopFile := range autoStartList {
		go func(desktopFile string) {
			delay, err := getDelayTime(desktopFile)
			if err != nil {
				logger.Warning(err)
			}

			if delay != 0 {
				time.Sleep(delay)
			}
			err = _startManager.launchAppWithOptions(desktopFile, 0, nil, nil)
			if err != nil {
				logger.Warning(err)
			}
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
