package video_card

import (
	"bytes"
	"fmt"
	"os/exec"

	"pkg.deepin.io/lib/log"
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

func supportRunGoodWM() bool {
	support := true

	platform, err := getPlatform()
	if err == nil && platform == platformSW {
		if !isRadeonExists() {
			support = false
		}
	}

	if !isDriverLoadedCorrectly() {
		return false
	}

	env, err := getVideoEnv()
	if err == nil {
		out, err := exec.Command("/sbin/lsmod").CombinedOutput()
		if err == nil {
			switch env {
			case envVirtualbox:
				if !bytes.Contains(out, []byte("vboxvideo")) {
					support = false
				}
			case envVmware:
				if !bytes.Contains(out, []byte("vmwgfx")) {
					support = false
				}
			}
		} else {
			fmt.Printf("warning: failed to exec lsmod: %v: %s", err, out)
		}
	}

	return support
}

const (
	workabilityUnknown = 0
	workabilityAble    = 1
	workabilityNotAble = 2
)

var _workability int

func SupportRunGoodWM() bool {
	switch _workability {
	case workabilityUnknown:
		support := supportRunGoodWM()
		if support {
			_workability = workabilityAble
		} else {
			_workability = workabilityNotAble
		}
		return support
	case workabilityAble:
		return true
	case workabilityNotAble:
		return false
	default:
		panic(fmt.Errorf("invalid workability %d", _workability))
	}
}
