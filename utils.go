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
	"dbus/com/deepin/dde/welcome"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"pkg.deepin.io/lib/utils"
	"strings"

	"bytes"
	"os/exec"
	"time"

	"gir/glib-2.0"
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

	num, err := f.GetInteger(glib.KeyFileDesktopGroup, GnomeDelayKey)
	if err != nil {
		logger.Debug("get", GnomeDelayKey, "failed", err)
		return 0
	}

	return time.Second * time.Duration(num)
}

func recordStartWMClass(o string, startupWMClass string) {
	logger.Info("startupWMClass")
	f := glib.NewKeyFile()
	defer f.Free()

	homePath := os.Getenv("HOME")
	filterDir := path.Join(homePath, ".config/dock")
	os.MkdirAll(filterDir, 0664)
	filterPath := path.Join(filterDir, "filter.ini")
	if !Exist(filterPath) {
		f, err := os.Create(filterPath)
		if err != nil {
			logger.Errorf("Launcher create config failedfailed: %s", err)
		} else {
			f.Close()
		}
	} else {
		if ok, err := f.LoadFromFile(
			filterPath,
			glib.KeyFileFlagsKeepComments|glib.KeyFileFlagsKeepTranslations,
		); !ok {
			logger.Errorf("Launcher load config failed: %s", err)
			return
		}

		basename := path.Base(o)
		dot := strings.LastIndex(
			basename,
			path.Ext(o),
		)
		appid := strings.Replace(
			basename[:dot],
			"_",
			"-",
			-1,
		)
		f.SetString(startupWMClass, "appid", appid)
		f.SetString(startupWMClass, "path", o)
		saveKeyFile(f, filterPath)
	}
}

func execAndWait(timeout int, name string, arg ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(name, arg...)
	var bufStdout, bufStderr bytes.Buffer
	cmd.Stdout = &bufStdout
	cmd.Stderr = &bufStderr
	err = cmd.Start()
	if err != nil {
		return
	}

	// wait for process finished
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		if err = cmd.Process.Kill(); err != nil {
			return
		}
		<-done
		err = fmt.Errorf("time out and process was killed")
	case err = <-done:
		stdout = bufStdout.String()
		stderr = bufStderr.String()
		if err != nil {
			return
		}
	}
	return
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

func getWindowManager() string {
	s, err := utils.CheckAndNewGSettings("com.deepin.dde.startdde")
	if err != nil {
		logger.Error("Failed to get window manager:", err)
		return ""
	}
	defer s.Unref()
	return s.GetString("wm-cmd")
}

func getAutostartDelay() int32 {
	s, err := utils.CheckAndNewGSettings("com.deepin.dde.startdde")
	if err != nil {
		logger.Error("Failed to get autostart delay:", err)
		return 0
	}
	defer s.Unref()
	return s.GetInt("autostart-delay")
}
