// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package wm

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/linuxdeepin/go-lib/strv"
	"regexp"
	"strconv"
	"strings"
)

const (
	envUnknown    int = 0
	envVirtualbox     = 1 << 0
	envVmware         = 1 << 1
	envIntel          = 1 << 2
	envAMD            = 1 << 3
	envNvidia         = 1 << 4
)

func getVideoEnv() (int, error) {
	outs, err := exec.Command("lspci").CombinedOutput()
	if err != nil {
		if len(outs) != 0 {
			err = fmt.Errorf("%s", string(outs))
		}
		return envUnknown, err
	}

	video := envUnknown
	switch {
	case regexp.MustCompile("vga.* virtualbox").Match(outs):
		video |= envVirtualbox
	case regexp.MustCompile("vga.* vmware").Match(outs):
		video |= envVmware
	case regexp.MustCompile("(vga|3d).* intel").Match(outs):
		video |= envIntel
	case regexp.MustCompile("(vga|3d).* ati").Match(outs):
		video |= envAMD
	case regexp.MustCompile("(vga|3d).* nvidia").Match(outs):
		video |= envNvidia
	}

	return video, nil
}

func correctWMByEnv(video int, good *bool) error {
	outs, err := exec.Command("/sbin/lsmod").CombinedOutput()
	if err != nil {
		if len(outs) != 0 {
			err = fmt.Errorf("%s", string(outs))
		}
		return err
	}

	//FIXME: check dual video cards and detect which is in use
	//by Xorg now.
	switch video {
	case envAMD:
		if strings.Contains(string(outs), "fglrx") && *good {
			os.Setenv("COGL_DRIVER", "gl")
		}
	case envNvidia:
		if strings.Contains(string(outs), "nvidia") { //nolint
			//TODO: still need to test and verify
		}
	case envVirtualbox:
		if !strings.Contains(string(outs), "vboxvideo") {
			*good = false
		}
	case envVmware:
		if !strings.Contains(string(outs), "vmwgfx") {
			*good = false
		}
	}
	return nil
}

func isDriverLoadedCorrectly() bool {
	fr, err := os.Open("/var/log/Xorg.0.log")
	if err != nil {
		return true
	}
	defer fr.Close()
	aiglxErr := regexp.MustCompile(`\(EE\)\s+AIGLX error`)
	driOk := regexp.MustCompile(`direct rendering: DRI\d+ enabled`)
	swrast := regexp.MustCompile("GLX: Initialized DRISWRAST")

	scanner := bufio.NewScanner(fr)
	for scanner.Scan() {
		data := scanner.Bytes()
		switch {
		case aiglxErr.Match(data):
			fmt.Println("Found agiglx error")
			return false
		case driOk.Match(data):
			fmt.Println("DRI enabled successfully")
			return true
		case swrast.Match(data):
			fmt.Println("swrast driver used")
			return false
		}
	}
	return true
}

func isRadeonDRI() bool {
	fmt.Println("DRM info is unreadable, try xdriinfo")
	outs, err := exec.Command("xdriinfo", "driver", "0").CombinedOutput()
	if err != nil {
		return false
	}

	lines := strings.Split(string(outs), "\n")
	tmp := strv.Strv(lines).FilterEmpty()
	if len(tmp) == 0 {
		return true
	}

	var list = strv.Strv{"r600", "r300", "r200", "radeon", "radeonsi"}
	return list.Contains(string(tmp[0]))
}

func isRadeonExists() bool {
	fmt.Println("Checking radeon card")
	var viables []int
	for i := 0; i < 4; i++ {
		if !isDeviceViable(i) {
			continue
		}
		viables = append(viables, i)
	}

	if len(viables) < 1 {
		return isRadeonDRI()
	}

	var drivers = []string{"radeon", "fglrx", "amdgpu"}
	return isCardExists(viables, drivers)
}

func isDeviceViable(id int) bool {
	//OK, on shenwei, this file may have no read permission for group/other.
	var filename = fmt.Sprintf("/sys/class/drm/card%d/device/enable", id)
	outs, err := ioutil.ReadFile(filename)
	if err != nil {
		return false
	}

	tmp := strv.Strv(strings.Split(string(outs), "\n")).FilterEmpty()
	if len(tmp) == 0 {
		return false
	}

	v, err := strconv.ParseInt(tmp[0], 10, 32)
	if err != nil {
		return false
	}

	// nouveau write 2, others 1
	return (v > 0)
}

func isCardExists(ids []int, drivers []string) bool {
	for _, id := range ids {
		filename := fmt.Sprintf("/sys/class/drm/card%d/device/driver", id)
		real, err := os.Readlink(filename)
		if err != nil {
			continue
		}

		name := filepath.Base(real)
		if strv.Strv(drivers).Contains(name) {
			return true
		}
	}
	return false
}
