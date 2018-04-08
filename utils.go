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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"dbus/com/deepin/dde/welcome"

	"gir/glib-2.0"
	"pkg.deepin.io/lib/utils"
	"pkg.deepin.io/lib/xdg/basedir"
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

func saveKeyFile(file *glib.KeyFile, path string) error {
	_, content, err := file.ToData()
	if err != nil {
		return err
	}

	stat, err := os.Lstat(path)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, []byte(content), stat.Mode())
	if err != nil {
		return err
	}
	return nil
}

func getDelayTime(o string) time.Duration {
	f := glib.NewKeyFile()
	defer f.Free()

	_, err := f.LoadFromFile(o, glib.KeyFileFlagsNone)
	if err != nil {
		logger.Warning("load", o, "failed:", err)
		return 0
	}

	num, err := f.GetInteger(glib.KeyFileDesktopGroup, KeyXGnomeAutostartDelay)
	if err != nil {
		logger.Debug("get", KeyXGnomeAutostartDelay, "failed", err)
		return 0
	}

	return time.Second * time.Duration(num)
}

var _ddeWelcome *welcome.Welcome

func getDDEWelcome() (*welcome.Welcome, error) {
	if _ddeWelcome != nil {
		return _ddeWelcome, nil
	}

	var err error
	_ddeWelcome, err = welcome.NewWelcome("com.deepin.dde.Welcome",
		"/com/deepin/dde/Welcome")
	return _ddeWelcome, err
}

func showWelcome(showing bool) error {
	wel, err := getDDEWelcome()
	if err != nil {
		return err
	}

	if showing {
		return wel.Show()
	}
	return wel.Exit()
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
	autoStartDelay    int32
	iowaitEnabled     bool
	memcheckerEnabled bool
	launchWelcome     bool
	swapSchedEnabled  bool
}

func getGSettingsConfig() *GSettingsConfig {
	gs, err := utils.CheckAndNewGSettings("com.deepin.dde.startdde")
	if err != nil {
		logger.Warning(err)
		// default values
		return &GSettingsConfig{
			autoStartDelay:    0,
			iowaitEnabled:     false,
			memcheckerEnabled: false,
			launchWelcome:     true,
			swapSchedEnabled:  false,
		}
	}
	cfg := &GSettingsConfig{
		autoStartDelay:    gs.GetInt("autostart-delay"),
		iowaitEnabled:     gs.GetBoolean("iowait-enabled"),
		memcheckerEnabled: gs.GetBoolean("memchecker-enabled"),
		launchWelcome:     gs.GetBoolean("launch-welcome"),
		swapSchedEnabled:  gs.GetBoolean("swap-sched-enabled"),
	}
	gs.Unref()
	return cfg
}
