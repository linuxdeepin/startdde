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
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"bytes"
	"os/exec"
	"time"

	"gir/gio-2.0"
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

func launch(name interface{}, list interface{}, timestamp uint32) error {
	switch o := name.(type) {
	case string:
		logger.Debug("string")
		if !strings.HasSuffix(o, ".desktop") {
			app, err := gio.AppInfoCreateFromCommandline(
				o,
				"",
				gio.AppInfoCreateFlagsNone,
			)
			if err != nil {
				return err
			}
			defer app.Unref()

			_, err = app.Launch(list.([]*gio.File), gio.GetGdkAppLaunchContext().SetTimestamp(timestamp))
			return err
		}

		var app *gio.DesktopAppInfo
		// maybe use AppInfoCreateFromCommandline with
		// AppInfoCreateFlagsSupportsStartupNotification flag
		if path.IsAbs(o) {
			logger.Debug("the path to launch is abs")
			app = gio.NewDesktopAppInfoFromFilename(o)
		} else {
			logger.Info("the path to launch is not abs")
			app = gio.NewDesktopAppInfo(o)
		}
		if app == nil {
			return errors.New("Launch failed")
		}
		defer app.Unref()

		startupWMClass := app.GetStartupWmClass()
		if startupWMClass != "" {
			recordStartWMClass(o, startupWMClass)
		}

		_, err := app.Launch(list.([]*gio.File), gio.GetGdkAppLaunchContext().SetTimestamp(timestamp))
		return err

	case *gio.AppInfo, *gio.DesktopAppInfo:
		_, err := name.(*gio.AppInfo).Launch(list.([]*gio.File), gio.GetGdkAppLaunchContext().SetTimestamp(timestamp))
		return err

	case *gio.File:
		return errors.New("not supported type now")
	}

	return errors.New("not suported type now")
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
