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
	"pkg.linuxdeepin.com/lib/gio-2.0"
	"pkg.linuxdeepin.com/lib/glib-2.0"
	"time"
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

func launch(name interface{}, list interface{}) error {
	switch o := name.(type) {
	case string:
		logger.Debug("string")
		if strings.HasSuffix(o, ".desktop") {
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

			startupWmClass := app.GetStartupWmClass()
			if startupWmClass != "" {
				logger.Info("startupWMClass")
				f := glib.NewKeyFile()
				defer f.Free()

				homePath := os.Getenv("HOME")
				filterPath := path.Join(
					homePath,
					"/.config/dock/filter.ini",
				)
				if !Exist(filterPath) {
					f, err := os.Create(filterPath)
					if err != nil {
						return fmt.Errorf("Launcher create config failedfailed: %s", err)
					}
					f.Close()
				}
				if ok, err := f.LoadFromFile(
					filterPath,
					glib.KeyFileFlagsNone,
				); !ok {
					return fmt.Errorf("Launcher load config failed: %s", err)
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
				f.SetString(startupWmClass, "appid", appid)
				f.SetString(startupWmClass, "path", o)
				saveKeyFile(f, filterPath)
			}

			// TODO: read delay field
			// TODO: launch context???
			_, err := app.Launch(list.([]*gio.File), nil)
			return err
		} else {
			app, err := gio.AppInfoCreateFromCommandline(
				o,
				"",
				gio.AppInfoCreateFlagsNone,
			)
			if err != nil {
				return err
			}

			defer app.Unref()

			_, err = app.Launch(list.([]*gio.File), nil)
			return err
		}

	case *gio.AppInfo, *gio.DesktopAppInfo:
		_, err := name.(*gio.AppInfo).Launch(list.([]*gio.File), nil)
		return err

	case *gio.File:
		return errors.New("not support now")
	}

	return errors.New("not suport")
}

func execCommand(cmd string, arg string) {
	err := exec.Command(cmd, arg).Run()
	if err != nil {
		logger.Errorf("Exec '%s %s' Failed: %s\n",
			cmd, arg, err)
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
