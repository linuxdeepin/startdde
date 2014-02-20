package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/howeyc/fsnotify"

	"dlib/dbus"
	"dlib/gio-2.0"
	"dlib/glib-2.0"
)

const (
	_OBJECT = "com.deepin.SessionManager"
	_PATH   = "/com/deepin/StartManager"
	_INTER  = "com.deepin.StartManager"
)

const (
	_AUTOSTART    = "autostart"
	DESKTOP_ENV   = "Deepin"
	HiddenKey     = "Hidden"
	OnlyShowInKey = "OnlyShowIn"
	NotShowInKey  = "NotShowIn"
	TryExecKey    = "TryExec"
)

var (
	c chan os.Signal
)

type StartManager struct {
	userAutostartPath string
	AutostartChanged  func(string, string)
}

func (m *StartManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{_OBJECT, _PATH, _INTER}
}

func (m *StartManager) Launch(name string) bool {
	list := make([]*gio.File, 0)
	err := launch(name, list)
	if err != nil {
		fmt.Println(err)
	}
	return err == nil
}

type AutostartInfo struct {
	renamed    chan bool
	created    chan bool
	notRenamed chan bool
	notCreated chan bool
}

func (m *StartManager) emitAutostartChanged(name, status string, info map[string]AutostartInfo) {
	m.AutostartChanged(status, name)
	delete(info, name)
}

func (m *StartManager) autostartHandler(ev *fsnotify.FileEvent, name string, info map[string]AutostartInfo) {
	// fmt.Println(ev)
	if _, ok := info[name]; !ok {
		info[name] = AutostartInfo{
			make(chan bool),
			make(chan bool),
			make(chan bool),
			make(chan bool),
		}
	}
	if ev.IsRename() {
		select {
		case <-info[name].renamed:
		default:
		}
		go func() {
			select {
			case <-info[name].notRenamed:
				return
			case <-time.After(time.Second):
				<-info[name].renamed
				m.emitAutostartChanged(name, "delete", info)
				// fmt.Println("deleted")
			}
		}()
		info[name].renamed <- true
	} else if ev.IsCreate() {
		go func() {
			select {
			case <-info[name].renamed:
				info[name].notRenamed <- true
				info[name].renamed <- true
			default:
			}
			select {
			case <-info[name].notCreated:
				return
			case <-time.After(time.Second):
				<-info[name].created
				m.emitAutostartChanged(name, "added", info)
				// fmt.Println("create added")
			}
		}()
		info[name].created <- true
	} else if ev.IsModify() && !ev.IsAttrib() {
		go func() {
			select {
			case <-info[name].created:
				info[name].notCreated <- true
			}
			select {
			case <-info[name].renamed:
				// fmt.Println("modified")
				m.emitAutostartChanged(name, "modified", info)
			default:
				m.emitAutostartChanged(name, "added", info)
				// fmt.Println("modify added")
			}
		}()
	} else if ev.IsAttrib() {
		go func() {
			select {
			case <-info[name].renamed:
				<-info[name].created
				info[name].notCreated <- true
			default:
			}
		}()
	} else if ev.IsDelete() {
		m.emitAutostartChanged(name, "deleted", info)
		// fmt.Println("deleted")
	}
}

func (m *StartManager) eventHandler(watcher *fsnotify.Watcher) {
	info := map[string]AutostartInfo{}
	for {
		select {
		case ev := <-watcher.Event:
			name := path.Clean(ev.Name)
			basename := path.Base(name)
			matched, _ := path.Match(`[^#.]*.desktop`, basename)
			if matched {
				if _, ok := info[name]; !ok {
					info[name] = AutostartInfo{
						make(chan bool, 1),
						make(chan bool, 1),
						make(chan bool, 1),
						make(chan bool, 1),
					}
				}
				m.autostartHandler(ev, name, info)
			}
		case <-watcher.Error:
		}
	}
}

func (m *StartManager) listenAutostart() {
	watcher, _ := fsnotify.NewWatcher()
	for _, dir := range m.autostartDirs() {
		watcher.Watch(dir)
	}
	go m.eventHandler(watcher)
}

type ActionGroup struct {
	Action     string
	ActionName string
}

func (m *StartManager) ListActions(name string) []ActionGroup {
	actions := make([]ActionGroup, 0)
	var o *gio.DesktopAppInfo
	if path.IsAbs(name) {
		o = gio.NewDesktopAppInfoFromFilename(name)
	} else {
		o = gio.NewDesktopAppInfo(name)
	}
	if o == nil {
		return actions
	}
	defer o.Unref()

	for _, action := range o.ListActions() {
		actionName := o.GetActionName(action)
		// fmt.Printf("%s, \"%s\"\n", action, actionName)
		actions = append(actions, ActionGroup{action, actionName})
	}
	return actions
}

func (m *StartManager) LaunchAction(name, action string) bool {
	var o *gio.DesktopAppInfo
	if path.IsAbs(name) {
		o = gio.NewDesktopAppInfoFromFilename(name)
	} else {
		o = gio.NewDesktopAppInfo(name)
	}
	if o == nil {
		fmt.Println("Create DesktopAppInfo failed")
		return false
	}
	defer o.Unref()

	for _, actionGroup := range m.ListActions(name) {
		if action == actionGroup.Action {
			o.LaunchAction(action, nil)
			return true
		}
	}

	fmt.Printf("Has no such a key '%s'\n", action)
	return false
}

func (m *StartManager) isUserAutostart(name string) bool {
	return path.Dir(path.Clean(name)) == m.getUserAutostartDir()
}

func (m *StartManager) isHidden(file *gio.DesktopAppInfo) bool {
	return file.HasKey(HiddenKey) && file.GetBoolean(HiddenKey)
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

func findExec(_path, cmd string, exist chan bool) {
	defer func() {
		recover()
	}()

	found := false
	filepath.Walk(
		_path,
		func(_path string, info os.FileInfo, err error) error {
			if info.Name() == cmd {
				found = true
				return errors.New("Found it")
			}
			return nil
		})
	exist <- found
}

func (m *StartManager) hasValidTryExecKey(file *gio.DesktopAppInfo) bool {
	// name := file.GetFilename()
	if !file.HasKey(TryExecKey) {
		// fmt.Println(name, "No TryExec Key")
		return true
	}

	cmd := file.GetString(TryExecKey)
	if cmd == "" {
		// fmt.Println(name, "TryExecKey is empty")
		return true
	}

	if path.IsAbs(cmd) {
		// fmt.Println(cmd, "is exist?", Exist(cmd))
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

	return m.hasValidTryExecKey(file) && !m.isHidden(file) && m.showInDeepin(file)
}

func lowerBaseName(name string) string {
	return strings.ToLower(path.Base(name))
}

func (m *StartManager) getUserStart(sys string) (userPath string) {
	if !Exist(m.getUserAutostartDir()) {
		return
	}
	filepath.Walk(
		m.getUserAutostartDir(),
		func(_path string, info os.FileInfo, err error) error {
			if lowerBaseName(sys) == strings.ToLower(info.Name()) {
				userPath = _path
				return errors.New("Found it")
			}
			return nil
		})

	return
}

func (m *StartManager) isAutostart(name string) bool {
	if !strings.HasSuffix(name, ".desktop") {
		return false
	}

	if !m.isUserAutostart(name) {
		userStart := m.getUserStart(name)
		if userStart != "" {
			return m.isAutostartAux(userStart)
		}
	}

	return m.isAutostartAux(name)
}

func (m *StartManager) getAutostartApps(dir string) []string {
	apps := make([]string, 0)
	filepath.Walk(
		dir,
		func(_path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				if m.isAutostart(_path) {
					apps = append(apps, _path)
				}
			}
			return nil
		})

	return apps
}

func (m *StartManager) getUserAutostartDir() string {
	if m.userAutostartPath == "" {
		configPath := glib.GetUserConfigDir()
		m.userAutostartPath = path.Join(configPath, _AUTOSTART)
	}

	if !Exist(m.userAutostartPath) {
		err := os.MkdirAll(m.getUserAutostartDir(), 0775)
		if err != nil {
			fmt.Println(fmt.Errorf("create user autostart dir failed: %s", err))
		}
	}

	return m.userAutostartPath
}

func (m *StartManager) autostartDirs() []string {
	dirs := make([]string, 0)

	dirs = append(dirs, m.getUserAutostartDir())

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
			apps = append(apps, m.getAutostartApps(dir)...)
		}
	}
	return apps
}

func (m *StartManager) doSetAutostart(name string, autostart bool) error {
	file := glib.NewKeyFile()
	defer file.Free()
	if ok, err := file.LoadFromFile(name, glib.KeyFileFlagsNone); !ok {
		fmt.Println(err)
		return err
	}

	fmt.Println("set autostart to", autostart)
	file.SetBoolean(
		glib.KeyFileDesktopGroup,
		HiddenKey,
		!autostart,
	)

	return saveKeyFile(file, name)
}

func (m *StartManager) setAutostart(name string, autostart bool) error {
	if autostart == m.isAutostart(name) {
		fmt.Println("is already done")
		return nil
	}

	dst := name
	if !m.isUserAutostart(name) {
		fmt.Println("not user's")
		dst = m.getUserStart(name)
		if !Exist(dst) {
			err := copyFile(name, dst, CopyFileNotKeepSymlink)
			if err != nil {
				return err
			}
		}
	}

	return m.doSetAutostart(dst, autostart)
}

func (m *StartManager) AddAutostart(name string) bool {
	err := m.setAutostart(name, true)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

func (m *StartManager) RemoveAutostart(name string) bool {
	if !path.IsAbs(name) {
		file := gio.NewDesktopAppInfo(name)
		if file == nil {
			return false
		}
		name = file.GetFilename()
		file.Unref()
	}
	err := m.setAutostart(name, false)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

func (m *StartManager) IsAutostart(name string) bool {
	if !path.IsAbs(name) {
		file := gio.NewDesktopAppInfo(name)
		if file == nil {
			fmt.Println(name, "is not a vaild desktop file.")
			return false
		}
		name = file.GetFilename()
		file.Unref()
	}
	return m.isAutostart(name)
}

func startStartManager() {
	m := StartManager{}
	if err := dbus.InstallOnSession(&m); err != nil {
		fmt.Println("Install StartManager Failed:", err)
	}
	m.listenAutostart()
	for _, name := range m.AutostartList() {
		// fmt.Println(name)
		if debug {
			continue
		}
		m.Launch(name)
	}
}
