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

package wm

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	platformX86 = iota + 1
	platformSW
	platformMIPS
	platformARM
	platformUnknown
)

func getPlatform() (int, error) {
	outs, err := exec.Command("uname", "-m").CombinedOutput()
	if err != nil {
		if len(outs) != 0 {
			err = fmt.Errorf("%s", string(outs))
		}
		return platformUnknown, err
	}

	str := strings.ToLower(string(outs))
	idxs := regexp.MustCompile("x86.*|i?86|ia64").FindStringIndex(str)
	if len(idxs) != 0 {
		return platformX86, nil
	}

	switch {
	case strings.Contains(str, "alpha"), strings.Contains(str, "sw_64"):
		return platformSW, nil
	case strings.Contains(str, "mips"):
		return platformMIPS, nil
	case strings.Contains(str, "arm"):
		return platformARM, nil
	}
	return platformUnknown, nil
}

func setupSWPlatform() error {
	os.Setenv("META_DEBUG_NO_SHADOW", "1")
	os.Setenv("META_IDLE_PAINT_MODE", "fixed")
	os.Setenv("META_IDLE_PAINT_FPS", "28")
	return reduceAnimations(true)
}

func reduceAnimations(v bool) error {
	args := []string{"set",
		"com.deepin.wrap.gnome.metacity",
		"reduced-resources"}
	if v {
		args = append(args, "true")
	} else {
		args = append(args, "false")
	}
	outs, err := exec.Command("gsettings", args...).CombinedOutput()
	if err != nil && len(outs) != 0 {
		err = fmt.Errorf("%s", string(outs))
	}
	return err
}
