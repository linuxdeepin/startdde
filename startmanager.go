package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	_AUTOSTART              = "autostart"
	GNOME_AUTOSTART_ENABLED = "X-GNOME-Autostart-enabled"
)

type StartManager struct {
	userAutostartPath string
}

func (m *StartManager) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{_OBJECT, _PATH, _INTER}
}

func (m *StartManager) Launch(name string) bool {
	list := make([]*gio.File, 0)
	err := Launch(name, list)
	if err != nil {
		fmt.Println(err)
	}
	return err == nil
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

func (m *StartManager) readXGnomeAutostartEnabled(name string) (bool, error) {
	file := gio.NewDesktopAppInfoFromFilename(name)
	if file == nil {
		return false, errors.New("Load failed")
	}
	defer file.Unref()

	if !file.HasKey(GNOME_AUTOSTART_ENABLED) {
		return false, errors.New("Not found key")
	}

	return file.GetBoolean(GNOME_AUTOSTART_ENABLED), nil
}

func (m *StartManager) isAutostartAux(name string) bool {
	autostart, err := m.readXGnomeAutostartEnabled(name)
	if err != nil {
		return true
	}
	return autostart
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

	if Exist(m.getUserAutostartDir()) {
		dirs = append(dirs, m.getUserAutostartDir())
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
		apps = append(apps, m.getAutostartApps(dir)...)
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
		GNOME_AUTOSTART_ENABLED,
		autostart,
	)

	_, content, err := file.ToData()
	if err != nil {
		return err
	}

	stat, err := os.Lstat(name)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(name, []byte(content), stat.Mode())
	if err != nil {
		return err
	}

	return nil
}

func (m *StartManager) setAutostart(name string, autostart bool) error {
	if autostart == m.isAutostart(name) {
		fmt.Println("is already done")
		return nil
	}

	dst := name
	if !m.isUserAutostart(name) {
		fmt.Println("not user")
		dst = m.getUserStart(name)
		err := CopyFile(name, dst, CopyFileNotKeepSymlink)
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
	for _, name := range m.AutostartList() {
		m.Launch(name)
	}
}
