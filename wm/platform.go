// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
