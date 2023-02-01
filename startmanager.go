// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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
	daemonApps "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.apps1"
	proxy "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.networkproxy1"
	systemPower "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/appinfo"
	"github.com/linuxdeepin/go-lib/appinfo/desktopappinfo"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/startdde/swapsched"
)

//go:generate dbusutil-gen em -type StartManager,SessionManager,Inhibitor

const (
	startManagerService   = "org.deepin.dde.StartManager1"
	startManagerObjPath   = "/org/deepin/dde/StartManager1"
	startManagerInterface = "org.deepin.dde.StartManager1"

	autostartDir      = "autostart"
	proxychainsBinary = "proxychains4"

	gSchemaLauncher        = "com.deepin.dde.launcher"
	gKeyAppsUseProxy       = "apps-use-proxy"
	gKeyAppsDisableScaling = "apps-disable-scaling"

	KeyXGnomeAutostartDelay = "X-GNOME-Autostart-Delay"
	KeyXGnomeAutoRestart    = "X-GNOME-AutoRestart"
	KeyXDeepinCreatedBy     = "X-Deepin-CreatedBy"
	KeyXDeepinAppID         = "X-Deepin-AppID"

	cpuFreqAdjustFile   = "/usr/share/startdde/app_startup.conf"
	performanceGovernor = "performance"

	restartRateLimitSeconds = 60

	dsettingsAppID                            = "org.deepin.startdde"
	dsettingsStartManagerName                 = "org.deepin.startdde.StartManager"
	dsettingsEnableSystemdApplicationUnitsKey = "enable-systemd-application-units"
)

type StartManager struct {
	xConn               *x.Conn
	service             *dbusutil.Service
	sysSigLoop          *dbusutil.SignalLoop
	userAutostartPath   string
	delayHandler        *mapDelayHandler
	daemonApps          daemonApps.Apps
	restartTimeMap      map[string]time.Time
	restartTimeMapMu    sync.Mutex
	proxyChainsConfFile string
	proxyChainsBin      string
	appsDir             []string
	settings            *gio.Settings
	appsUseProxy        strv.Strv
	appsDisableScaling  strv.Strv
	mu                  sync.Mutex
	appProxy            proxy.App

	NeededMemory     uint64
	systemPower      systemPower.Power
	cpuFreqAdjustMap map[string]int32

	userSystemd systemd1.Manager

	enableSystemdApplicationUnit bool

	//nolint
	signals *struct {
		AutostartChanged struct {
			status string
			name   string
		}
	}
}

func getLaunchedHooks(dir string) (ret []string) {
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

func (m *StartManager) getCpuFreqAdjustMap(path string) map[string]int32 {
	cpuFreqAdjustMap := make(map[string]int32)

	//the content format of each line is fixed: events timeout
	fi, err := os.Open(path)
	if err != nil {
		logger.Warning("open dde_startup.conf failed:", err)
		return nil
	}
	defer fi.Close()

	br := bufio.NewReader(fi)
	for {
		data, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		//retrieve data: arr[0] --> events
		//retrieve data: arr[1] --> timeout
		arr := strings.Split(string(data), " ")
		if len(arr) == 2 {
			//get the name of the binary file
			event := arr[0]
			locktime, _ := strconv.ParseInt(arr[1], 10, 32)
			cpuFreqAdjustMap[event] = int32(locktime)
		}
	}
	return cpuFreqAdjustMap
}

func (m *StartManager) enableCpuFreqLock(desktopFile string) error {
	fileName := filepath.Base(desktopFile)
	event := strings.TrimSuffix(fileName, ".desktop")
	value, ok := m.cpuFreqAdjustMap[event]

	if ok {
		err := m.systemPower.LockCpuFreq(0, performanceGovernor, value)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("application is not in app_startup.conf")
	}
	return nil
}

func newStartManager(xConn *x.Conn, service *dbusutil.Service) *StartManager {
	m := &StartManager{
		service: service,
		xConn:   xConn,
	}

	m.appsDir = getAppDirs()
	m.settings = gio.NewSettings(gSchemaLauncher)
	m.appsUseProxy = m.settings.GetStrv(gKeyAppsUseProxy)
	m.appsDisableScaling = m.settings.GetStrv(gKeyAppsDisableScaling)
	m.userSystemd = systemd1.NewManager(service.Conn())

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

	m.proxyChainsConfFile = filepath.Join(basedir.GetUserConfigDir(), "deepin", "proxychains.conf")
	m.proxyChainsBin, _ = exec.LookPath(proxychainsBinary)
	logger.Debugf("startManager proxychain confFile %q, bin: %q", m.proxyChainsConfFile, m.proxyChainsBin)

	m.restartTimeMap = make(map[string]time.Time)
	m.delayHandler = newMapDelayHandler(100*time.Millisecond,
		m.emitSignalAutostartChanged)
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
	}

	m.sysSigLoop = dbusutil.NewSignalLoop(sysBus, 10)
	m.sysSigLoop.Start()

	m.daemonApps = daemonApps.NewApps(sysBus)
	m.systemPower = systemPower.NewPower(sysBus)
	m.appProxy = proxy.NewApp(sysBus)
	m.cpuFreqAdjustMap = m.getCpuFreqAdjustMap(cpuFreqAdjustFile)
	m.initDSettings(sysBus)
	return m
}

func (m *StartManager) initDSettings(sysBus *dbus.Conn) {
	ds := configManager.NewConfigManager(sysBus)

	startManagerPath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsStartManagerName, "")
	if err != nil {
		logger.Warning(err)
		return
	}

	dsStartManager, err := configManager.NewManager(sysBus, startManagerPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	getEnableSystemdApplicationUnit := func() {
		v, err := dsStartManager.Value(0, dsettingsEnableSystemdApplicationUnitsKey)
		if err != nil {
			logger.Warning(err)
			return
		}

		m.enableSystemdApplicationUnit = v.Value().(bool)
	}

	getEnableSystemdApplicationUnit()

	dsStartManager.InitSignalExt(m.sysSigLoop, true)
	_, err = dsStartManager.ConnectValueChanged(func(key string) {
		if key == dsettingsEnableSystemdApplicationUnitsKey {
			getEnableSystemdApplicationUnit()
		}
	})
	if err != nil {
		logger.Warning(err)
	}
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
	return nil, dbusutil.ToError(errors.New("swap-sched disabled"))
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

	err = m.launchApp(desktopFile, timestamp, files, options)
	if err != nil {
		logger.Warning("launch failed:", err)
	}

	// mark app launched
	if m.daemonApps != nil {
		err := m.daemonApps.LaunchedRecorder().MarkLaunched(0, desktopFile)
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

	err = m.launchAppActionAux(desktopFile, action, timestamp)
	if err != nil {
		logger.Warning("launch failed:", err)
	}
	// mark app launched
	if m.daemonApps != nil {
		err := m.daemonApps.LaunchedRecorder().MarkLaunched(0, desktopFile)
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

	cmd := exec.Command(exe, args...)

	if dirVar, ok := options["dir"]; ok {
		if dirStr, ok := dirVar.Value().(string); ok {
			cmd.Dir = dirStr
		} else {
			return errors.New("type of option dir is not string")
		}
	}

	err = cmd.Start()
	return m.waitCmd(nil, cmd, err, _name)
}

func (m *StartManager) getAppIdByFilePath(file string) string {
	return getAppIdByFilePath(file, m.appsDir)
}

func (m *StartManager) shouldUseProxy(id string) bool {
	// TODO: add support for application proxy
	if m.enableSystemdApplicationUnit {
		return false
	}

	m.mu.Lock()
	if !m.appsUseProxy.Contains(id) {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()

	msg, err := m.appProxy.GetProxy(0)
	if err != nil {
		logger.Warningf("cant get proxy, err: %v", err)
		return false
	}
	if msg == "" {
		logger.Debug("dont have proxy settings, will not use proxy")
		return false
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

func (m *StartManager) createSystemdUnitForPID(appID string, desktopFile string, pid uint) {
	if appID == "" {
		appID = strings.TrimSuffix(filepath.Base(desktopFile), ".desktop")
	}

	unitName := fmt.Sprintf("app-dde-%s-%d.scope", appID, pid)

	properties := []systemd1.Property{
		{
			Name:  "Description",
			Value: dbus.MakeVariant("Launched by DDE"),
		},
		{
			Name:  "PIDs",
			Value: dbus.MakeVariant([]uint{pid}),
		},
		{
			Name:  "CollectMode",
			Value: dbus.MakeVariant("inactive-or-failed"),
		},
	}

	m.userSystemd.StartTransientUnit(0, unitName, "fail", properties, nil)
}

func (m *StartManager) launch(appInfo *desktopappinfo.DesktopAppInfo, timestamp uint32,
	files []string, iStartCmd IStartCommand, cmdName string) error {
	desktopFile := appInfo.GetFileName()
	logger.Debug("launch: desktopFile is", desktopFile)
	var err error
	var cmdPrefixes []string

	err = m.enableCpuFreqLock(desktopFile)
	if err != nil {
		logger.Debug("cpu freq lock failed:", err)
	}

	appId := m.getAppIdByFilePath(desktopFile)
	if appId != "" {
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

	ctx := appinfo.NewAppLaunchContext(m.xConn)
	ctx.SetTimestamp(timestamp)
	if len(cmdPrefixes) > 0 {
		logger.Debug("cmd prefixes:", cmdPrefixes)
		ctx.SetCmdPrefixes(cmdPrefixes)
	}

	if appInfo.IsDesktopOverrideExecSet() {
		logger.Debug("cmd override exec:", appInfo.GetDesktopOverrideExec())
	}

	logger.Infof("app id %v check use app proxy", appInfo.GetId())
	if m.shouldUseProxy(appInfo.GetId()) {
		env := removeProxy(os.Environ())
		logger.Infof("app %v use app proxy, clear proxy env, env: %v", appInfo.GetId(), env)
		ctx.SetEnv(env)
	}

	cmd, err := iStartCmd.StartCommand(files, ctx)

	if m.enableSystemdApplicationUnit {
		m.createSystemdUnitForPID(appId, desktopFile, uint(cmd.Process.Pid))
	}

	return m.waitCmd(appInfo, cmd, err, cmdName)
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

func (m *StartManager) launchApp(desktopFile string, timestamp uint32, files []string, options map[string]dbus.Variant) error {
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

	if execVar, ok := options["desktop-override-exec"]; ok {
		execStr, isStr := execVar.Value().(string)
		if !isStr {
			return errors.New("type of option desktop-override-exec is not string")
		}
		appInfo.SetDesktopOverrideExec(execStr)
	}

	return m.launch(appInfo, timestamp, files, appInfo, desktopFile)
}

func (m *StartManager) launchAppActionAux(desktopFile, actionSection string, timestamp uint32) error {
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

	return m.launch(appInfo, timestamp, nil, &targetAction, desktopFile+actionSection)
}

func (m *StartManager) waitCmd(appInfo *desktopappinfo.DesktopAppInfo, cmd *exec.Cmd, err error, cmdName string) error {
	if err != nil {
		return err
	}

	go func() {
		// check if should use new proxy
		// check if app info is empty
		if appInfo != nil {
			appId := appInfo.GetId()
			logger.Infof("current appId is %s", appId)
			if m.shouldUseProxy(appId) {
				pid := cmd.Process.Pid
				logger.Infof("should use proxy, %v", pid)
				err = m.appProxy.AddProc(0, int32(pid))
				if err != nil {
					logger.Warningf("add proc failed, err: %v", err)
				}
			}
		}
		err := cmd.Wait()
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
						err = m.launch(appInfo, 0, nil, appInfo, appInfo.GetFileName())
						if err != nil {
							logger.Warningf("failed to restart app %q", appInfo.GetFileName())
						}
						m.setRestartTime(appInfo, now)
					}
				}
			}
		}
	}()

	return nil
}

func removeProxy(sl []string) []string {
	result := removeSl(sl, "auto_proxy")
	result = removeSl(result, "http_proxy")
	result = removeSl(result, "https_proxy")
	result = removeSl(result, "ftp_proxy")
	result = removeSl(result, "all_proxy")
	result = removeSl(result, "SOCKS_SERVER")
	result = removeSl(result, "no_proxy")
	return result
}

func removeSl(sl []string, mem string) []string {
	for index, elem := range sl {
		if !strings.HasPrefix(elem, mem) {
			continue
		}
		sl = append(sl[:index], sl[index+1:]...)
		break
	}
	return sl
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

func startStartManager(xConn *x.Conn, service *dbusutil.Service) {
	_startManager = newStartManager(xConn, service)
	err := service.Export(startManagerObjPath, _startManager)
	if err != nil {
		logger.Warning("export StartManager failed:", err)
	}
	err = service.RequestName(startManagerService)
	if err != nil {
		logger.Warning("export StartManager service failed:", err)
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
