// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	dbus "github.com/godbus/dbus/v5"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/appinfo/desktopappinfo"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

func Exist(name string) bool {
	_, err := os.Stat(name)
	return err == nil || os.IsExist(err)
}

type CopyFlag int

const (
	CopyFileNone CopyFlag = 1 << iota
	CopyFileNotKeepSymlink
	CopyFileOverWrite
)

func copyFileAux(src, dst string, copyFlag CopyFlag) error {
	srcStat, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("Error os.Lstat src %s: %s", src, err)
	}

	if (copyFlag&CopyFileOverWrite) != CopyFileOverWrite && Exist(dst) {
		return fmt.Errorf("error dst file is already exist")
	}

	os.Remove(dst)
	if (copyFlag&CopyFileNotKeepSymlink) != CopyFileNotKeepSymlink &&
		(srcStat.Mode()&os.ModeSymlink) == os.ModeSymlink {
		readlink, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("error read symlink %s: %s", src,
				err)
		}

		err = os.Symlink(readlink, dst)
		if err != nil {
			return fmt.Errorf("error creating symlink %s to %s: %s",
				readlink, dst, err)
		}
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening src file %s: %s", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(
		dst,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		srcStat.Mode(),
	)
	if err != nil {
		return fmt.Errorf("error opening dst file %s: %s", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error in copy from %s to %s: %s", src, dst,
			err)
	}

	return nil
}

func copyFile(src, dst string, copyFlag CopyFlag) error {
	srcStat, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("error os.Stat src %s: %s", src, err)
	}

	if srcStat.IsDir() {
		return fmt.Errorf("error src is a directory: %s", src)
	}

	if Exist(dst) {
		dstStat, err := os.Lstat(dst)
		if err != nil {
			return fmt.Errorf("error os.Lstat dst %s: %s", dst, err)
		}

		if dstStat.IsDir() {
			dst = path.Join(dst, path.Base(src))
		} else {
			if (copyFlag & CopyFileOverWrite) == 0 {
				return fmt.Errorf("error dst %s is alreadly exist", dst)
			}
		}
	}

	return copyFileAux(src, dst, copyFlag)
}

func syncFile(filename string) error {
	fh, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fh.Close()
	return fh.Sync()
}

func getDelayTime(desktopFile string) (time.Duration, error) {
	dai, err := desktopappinfo.NewDesktopAppInfoFromFile(desktopFile)
	if err != nil {
		return 0, err
	}

	num, _ := dai.GetInt(desktopappinfo.MainSection, KeyXGnomeAutostartDelay)

	return time.Second * time.Duration(num), nil
}

func showDDEWelcome() error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	obj := systemBus.Object("com.deepin.ABRecovery", "/com/deepin/ABRecovery")
	var canRestore bool
	err = obj.Call("com.deepin.ABRecovery.CanRestore", 0).Store(&canRestore)
	if err != nil {
		return err
	}
	if canRestore {
		return nil
	}
	cmd := exec.Command("/usr/lib/deepin-daemon/dde-welcome")
	err = cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning(err)
		}
	}()

	return nil
}

const (
	AppDirName = "applications"
	desktopExt = ".desktop"
)

func getAppDirs() []string {
	dataDirs := basedir.GetSystemDataDirs()
	dataDirs = append(dataDirs, basedir.GetUserDataDir())
	var dirs []string
	for _, dir := range dataDirs {
		dirs = append(dirs, path.Join(dir, AppDirName))
	}
	return dirs
}

func getAppIdByFilePath(file string, appDirs []string) string {
	file = filepath.Clean(file)
	var desktopId string
	for _, dir := range appDirs {
		if strings.HasPrefix(file, dir) {
			desktopId, _ = filepath.Rel(dir, file)
			break
		}
	}
	if desktopId == "" {
		return ""
	}
	return strings.TrimSuffix(desktopId, desktopExt)
}

type GSettingsConfig struct {
	autoStartDelay       int32
	memcheckerEnabled    bool
	swapSchedEnabled     bool
	wmCmd                string
	needQuickBlackScreen bool
}

func getGSettingsConfig() *GSettingsConfig {
	gs := gio.NewSettings("com.deepin.dde.startdde")
	cfg := &GSettingsConfig{
		autoStartDelay:       gs.GetInt("autostart-delay"),
		memcheckerEnabled:    gs.GetBoolean("memchecker-enabled"),
		swapSchedEnabled:     gs.GetBoolean("swap-sched-enabled"),
		wmCmd:                gs.GetString("wm-cmd"),
		needQuickBlackScreen: gs.GetBoolean("quick-black-screen"),
	}
	gs.Unref()
	return cfg
}

func initGSettingsConfig() {
	if _gSettingsConfig == nil {
		_gSettingsConfig = getGSettingsConfig()
	}
}

func getLightDMAutoLoginUser() (string, error) {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/lightdm/lightdm.conf")
	if err != nil {
		return "", err
	}

	v, err := kf.GetString("Seat:*", "autologin-user")
	return v, err
}
