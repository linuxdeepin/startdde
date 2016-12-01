package display

import (
	"encoding/json"
	"os/exec"
	"pkg.deepin.io/dde/api/dxinput"
	"pkg.deepin.io/dde/api/dxinput/utils"
	"sort"
)

type uint16Splice []uint16

func (s uint16Splice) Len() int {
	return len(s)
}

func (s uint16Splice) Less(i, j int) bool {
	return s[i] < s[j]
}

func (s uint16Splice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s uint16Splice) equal(list uint16Splice) bool {
	sort.Sort(list)
	i, j := len(s), len(list)
	if i != j {
		return false
	}
	for v := 0; v < i; v++ {
		if s[v] != list[v] {
			return false
		}
	}
	return true
}

func rotateInputPointor(rotation uint16) {
	devs := utils.ListDevice()
	for _, dev := range devs {
		if dev.Type == utils.DevTypeMouse {
			m, _ := dxinput.NewMouseFromDeviceInfo(dev)
			m.SetRotation(uint8(rotation))
		}
		if dev.Type == utils.DevTypeTouchpad {
			tpad, _ := dxinput.NewTouchpadFromDevInfo(dev)
			tpad.SetRotation(uint8(rotation))
		}
	}
}

func doAction(cmd string) error {
	logger.Debug("Command:", cmd)
	return exec.Command("/bin/sh", "-c", "exec "+cmd).Run()
}

func jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func jsonUnmarshal(data string, ret interface{}) error {
	return json.Unmarshal([]byte(data), ret)
}
