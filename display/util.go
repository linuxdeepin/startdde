package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/lib/utils"
)

func getRotations(origin uint16) []uint16 {
	var ret []uint16

	if origin&randr.RotationRotate0 == randr.RotationRotate0 {
		ret = append(ret, randr.RotationRotate0)
	}
	if origin&randr.RotationRotate90 == randr.RotationRotate90 {
		ret = append(ret, randr.RotationRotate90)
	}
	if origin&randr.RotationRotate180 == randr.RotationRotate180 {
		ret = append(ret, randr.RotationRotate180)
	}
	if origin&randr.RotationRotate270 == randr.RotationRotate270 {
		ret = append(ret, randr.RotationRotate270)
	}
	return ret
}

func getReflects(origin uint16) []uint16 {
	var ret = []uint16{0}

	if origin&randr.RotationReflectX == randr.RotationReflectX {
		ret = append(ret, randr.RotationReflectX)
	}
	if origin&randr.RotationReflectY == randr.RotationReflectY {
		ret = append(ret, randr.RotationReflectY)
	}
	if len(ret) == 3 {
		ret = append(ret, randr.RotationReflectX|randr.RotationReflectY)
	}
	return ret
}

func parseCrtcRotation(origin uint16) (rotation, reflect uint16) {
	rotation = origin & 0xf
	reflect = origin & 0xf0

	switch rotation {
	case 1, 2, 4, 8:
		break
	default:
		//Invalid rotation value
		rotation = 1
	}

	switch reflect {
	case 0, 16, 32, 48:
		break
	default:
		// Invalid reflect value
		reflect = 0
	}

	return
}

func toModeInfo(info randr.ModeInfo) ModeInfo {
	return ModeInfo{
		Id:     info.Id,
		Width:  info.Width,
		Height: info.Height,
		Rate:   calcModeRate(info),
	}
}

func calcModeRate(info randr.ModeInfo) float64 {
	vTotal := float64(info.VTotal)
	if (info.ModeFlags & randr.ModeFlagDoubleScan) != 0 {
		/* doublescan doubles the number of lines */
		vTotal *= 2
	}
	if (info.ModeFlags & randr.ModeFlagInterlace) != 0 {
		/* interlace splits the frame into two fields */
		/* the field rate is what is typically reported by monitors */
		vTotal /= 2
	}

	if info.HTotal == 0 || vTotal == 0 {
		return 0
	} else {
		return float64(info.DotClock) / (float64(info.HTotal) * vTotal)
	}
}

func outputSliceContains(outputs []randr.Output, output randr.Output) bool {
	for _, o := range outputs {
		if o == output {
			return true
		}
	}
	return false
}

func getMonitorsCommonSizes(monitors []*Monitor) []Size {
	count := make(map[Size]int)
	for _, monitor := range monitors {
		smm := getSizeModeMap(monitor.Modes)
		for size := range smm {
			count[size]++
		}
	}
	var commonSizes []Size
	for size, value := range count {
		if value == len(monitors) {
			commonSizes = append(commonSizes, size)
		}
	}
	return commonSizes
}

func getMaxAreaSize(sizes []Size) Size {
	if len(sizes) == 0 {
		return Size{}
	}
	maxS := sizes[0]
	for _, s := range sizes[1:] {
		if (int(maxS.width) * int(maxS.height)) < (int(s.width) * int(s.height)) {
			maxS = s
		}
	}
	return maxS
}

type Size struct {
	width  uint16
	height uint16
}

func getSizeModeMap(modes []ModeInfo) map[Size][]uint32 {
	result := make(map[Size][]uint32)
	for _, modeInfo := range modes {
		result[Size{modeInfo.Width, modeInfo.Height}] = append(
			result[Size{modeInfo.Width, modeInfo.Height}], modeInfo.Id)
	}
	return result
}

func getCrtcRect(crtcInfo *randr.GetCrtcInfoReply) x.Rectangle {
	rect := x.Rectangle{
		X:      crtcInfo.X,
		Y:      crtcInfo.Y,
		Width:  crtcInfo.Width,
		Height: crtcInfo.Height,
	}
	return rect
}

func getOutputEDID(conn *x.Conn, output randr.Output) ([]byte, error) {
	atomEDID, err := conn.GetAtom("EDID")
	if err != nil {
		return nil, err
	}

	reply, err := randr.GetOutputProperty(conn, output,
		atomEDID, x.AtomInteger,
		0, 32, false, false).Reply(conn)
	if err != nil {
		return nil, err
	}
	return reply.Value, nil
}

var numReg = regexp.MustCompile(`-?[0-9]`)

func getOutputUUID(name string, edid []byte) string {
	if len(edid) < 128 {
		return name
	}

	id, _ := utils.SumStrMd5(string(edid[:128]))
	if id == "" {
		return name
	}
	return numReg.ReplaceAllString(name, "") + id
}

func sortMonitorsByID(monitors []*Monitor) {
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].ID < monitors[j].ID
	})
}

func getMinIDMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}

	minMonitor := monitors[0]
	for _, monitor := range monitors[1:] {
		if minMonitor.ID > monitor.ID {
			minMonitor = monitor
		}
	}
	return minMonitor
}

func jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func jsonUnmarshal(data string, ret interface{}) error {
	return json.Unmarshal([]byte(data), ret)
}

// see also: gnome-desktop/libgnome-desktop/gnome-rr.c
//           '_gnome_rr_output_name_is_builtin_display'
func isBuiltinOutput(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "vga"):
		return false
	case strings.HasPrefix(name, "hdmi"):
		return false
	case strings.HasPrefix(name, "dvi"):
		return false

	case strings.HasPrefix(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		return true
	case strings.HasPrefix(name, "lcd"):
		// fglrx uses "LCD" in some versions
		return true
	case strings.HasPrefix(name, "edp"):
		// eDP is for internal built-in panel connections
		return true
	case strings.HasPrefix(name, "dsi"):
		return true
	case name == "default":
		return true
	}
	return true
}

func doAction(cmd string) error {
	logger.Debug("Command:", cmd)
	c := exec.Command("/bin/sh", "-c", "exec "+cmd)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	err := c.Run()
	if err != nil {
		return fmt.Errorf("%s, stdErr: %s", err.Error(), errBuf.Bytes())
	}
	return nil
}

func needSwapWidthHeight(rotation uint16) bool {
	return rotation&randr.RotationRotate90 != 0 ||
		rotation&randr.RotationRotate270 != 0
}

func getConfigVersion(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(content)), nil
}
