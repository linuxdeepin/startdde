package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"dlib/gio-2.0"
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

func copyFile(src, dst string, copyFlag CopyFlag) error {
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

func CopyFile(src, dst string, copyFlag CopyFlag) error {
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

	return copyFile(src, dst, copyFlag)
}

func Launch(name interface{}, list interface{}) error {
	switch o := name.(type) {
	case string:
		fmt.Println("string")
		if strings.HasSuffix(o, ".desktop") {
			var app *gio.DesktopAppInfo
			// maybe use AppInfoCreateFromCommandline with
			// AppInfoCreateFlagsSupportsStartupNotification flag
			if path.IsAbs(o) {
				app = gio.NewDesktopAppInfoFromFilename(o)
			} else {
				app = gio.NewDesktopAppInfo(o)
			}
			if app == nil {
				return errors.New("Launch failed")
			}
			defer app.Unref()

			// TODO: read delay field
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
