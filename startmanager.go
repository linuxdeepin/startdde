package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/howeyc/fsnotify"

	"dlib/dbus"
	"dlib/gio-2.0"
	"dlib/glib-2.0"
)

const (
	_OBJECT = "com.deepin.StartManager"
	_PATH   = "/com/deepin/StartManager"
	_INTER  = _OBJECT
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

func (m *StartManager) emitAutostartChanged(
	ev *fsnotify.FileEvent,
	name string,
	renamed,
	created,
	notRenamed,
	notCreated chan bool) {
	// fmt.Println(ev)
	if ev.IsRename() {
		select {
		case <-renamed:
		default:
		}
		renamed <- true
		go func() {
			select {
			case <-notRenamed:
				return
			case <-time.After(time.Second):
				<-renamed
				m.AutostartChanged("deleted", name)
				// fmt.Println("deleted")
			}
		}()
	} else if ev.IsCreate() {
		created <- true
		go func() {
			select {
			case <-renamed:
				notRenamed <- true
				renamed <- true
			default:
			}
			select {
			case <-notCreated:
				return
			case <-time.After(time.Second):
				<-created
				m.AutostartChanged("added", name)
				// fmt.Println("create added")
			}
		}()
	} else if ev.IsModify() && !ev.IsAttrib() {
		go func() {
			select {
			case <-created:
				notCreated <- true
			}
			select {
			case <-renamed:
				// fmt.Println("modified")
				m.AutostartChanged("modified", name)
			default:
				m.AutostartChanged("added", name)
				// fmt.Println("modify added")
			}
		}()
	} else if ev.IsAttrib() {
		go func() {
			select {
			case <-renamed:
				<-created
				notCreated <- true
			default:
			}
		}()
	} else if ev.IsDelete() {
		m.AutostartChanged("deleted", name)
		// fmt.Println("deleted")
	}
}

func (m *StartManager) autostartHandler(watcher *fsnotify.Watcher) {
	renamed := make(chan bool, 1)
	created := make(chan bool, 1)
	notRenamed := make(chan bool, 1)
	notCreated := make(chan bool, 1)
	for {
		select {
		case ev := <-watcher.Event:
			name := path.Clean(ev.Name)
			basename := path.Base(name)
			matched, _ := path.Match(`[^#.]*.desktop`, basename)
			if matched {
				m.emitAutostartChanged(
					ev,
					name,
					renamed,
					created,
					notRenamed,
					notCreated,
				)
			}
		}
	}
}

func (m *StartManager) listenAutostart() {
	watcher, _ := fsnotify.NewWatcher()
	for _, dir := range m.autostartDirs() {
		watcher.Watch(dir)
	}
	go m.autostartHandler(watcher)

	c = make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, os.Kill)
	go func() { <-c; watcher.Close() }()
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

	return m.userAutostartPath
}

func (m *StartManager) autostartDirs() []string {
	dirs := make([]string, 0)

	// if Exist(m.getUserAutostartDir()) {
	dirs = append(dirs, m.getUserAutostartDir())
	// }

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
		err := copyFile(name, dst, CopyFileNotKeepSymlink)
		if err != nil {
			return err
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
	err := m.setAutostart(name, false)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

func startStartManager() {
	m := StartManager{}
	if err := dbus.InstallOnSession(&m); err != nil {
		fmt.Println("Install StartManager Failed:", err)
	}
	m.listenAutostart()
	for _, name := range m.AutostartList() {
		// fmt.Println(name)
		// continue
		m.Launch(name)
	}
}
