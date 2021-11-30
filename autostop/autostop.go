/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package autostop

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"github.com/linuxdeepin/go-lib/log"
)

var logger *log.Logger

func LaunchAutostopScripts(log *log.Logger) error {
	if log == nil {
		return fmt.Errorf("Logger is nil")
	}

	logger = log

	var dirs = []string{
		path.Join(os.Getenv("HOME"), ".config", "autostop"),
		"/etc/xdg/autostop",
	}

	errs := launchScripts(getScripts(dirs))
	for _, err := range errs {
		logger.Warning(err)
	}

	return nil
}

func launchScripts(scripts []string) []error {
	errs := []error{}
	for _, script := range scripts {
		logger.Info("[Autostop] will launch:", script)
		out, err := exec.Command(script).CombinedOutput()
		if err != nil {
			errs = append(errs, errors.New(fmt.Sprintf("[Autostop] failed to launch %s: %v, %v",
				script, string(out), err)))
		}
	}
	return errs
}

func getScripts(dirs []string) []string {
	var scripts []string
	for _, dir := range dirs {
		tmp, err := doScanScripts(dir)
		if err != nil {
			logger.Warning("[Autostop] failed to scan dir:", dir, err)
			continue
		}
		scripts = append(scripts, tmp...)
	}
	return scripts
}

func doScanScripts(dir string) ([]string, error) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scripts []string
	for _, finfo := range finfos {
		if finfo.IsDir() ||
			(finfo.Mode().Perm()&os.FileMode(0111) == 0) {
			continue
		}
		scripts = append(scripts, path.Join(dir, finfo.Name()))
	}
	return scripts, nil
}
