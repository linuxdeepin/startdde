package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	hostname1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.hostname1"
	dbus "pkg.deepin.io/lib/dbus1"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/gir/gudev-1.0"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/strv"
	"pkg.deepin.io/lib/utils"
)

const (
	filterFilePath = "/usr/share/startdde/filter.conf"
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
		name:   info.Name,
		Width:  info.Width,
		Height: info.Height,
		Rate:   calcModeRate(info),
	}
}

var regMode = regexp.MustCompile(`^(\d+)x(\d+)(\D+)$`)

func filterModeInfos(modes []ModeInfo) []ModeInfo {
	result := make([]ModeInfo, 0, len(modes))
	var filteredModeNames strv.Strv
	for idx := range modes {
		mode := modes[idx]
		skip := false

		if filteredModeNames.Contains(mode.name) {
			skip = true
		} else {
			match := regMode.FindStringSubmatch(mode.name)
			if match != nil {
				m := findFirstMode(modes, func(mode1 ModeInfo) bool {
					return mode.Width == mode1.Width &&
						mode.Height == mode1.Height &&
						len(mode1.name) > 0 &&
						isDigit(mode1.name[len(mode1.name)-1])
				})
				if m != nil {
					// 找到大小相同的 mode
					skip = true
					filteredModeNames = append(filteredModeNames, mode.name)
				}
			}

			if !skip {
				m := findFirstMode(result, func(mode1 ModeInfo) bool {
					return mode.Width == mode1.Width &&
						mode.Height == mode1.Height &&
						formatRate(mode.Rate) == formatRate(mode1.Rate)
				})
				if m != nil {
					//logger.Debugf("compare mode: %s, find m: %s",
					//	spew.Sdump(mode), spew.Sdump(m))
					skip = true
				}
			}
		}

		if skip {
			//logger.Debugf("filterModeInfos skip mode %d|%x %s %.2f", mode.Id, mode.Id, mode.name, mode.Rate)
		} else {
			//logger.Debugf("add mode %d|%x %s %.2f", mode.Id, mode.Id, mode.name, mode.Rate)
			result = append(result, mode)
		}
	}
	return result
}

func formatRate(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func findFirstMode(modes []ModeInfo, fn func(mode ModeInfo) bool) *ModeInfo {
	for _, mode := range modes {
		if fn(mode) {
			return &mode
		}
	}
	return nil
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

var numReg = regexp.MustCompile(`-?[0-9]`)

func getOutputUUID(name string, edid []byte) string {
	if len(edid) < 128 {
		return name
	}

	id, _ := utils.SumStrMd5(string(edid[:128]))
	if id == "" {
		return name
	}
	return name + id
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

// 获取最早连接的显示器
func getMinLastConnectedTimeMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}
	minMonitor := monitors[0]
	for _, monitor := range monitors[1:] {
		if minMonitor.lastConnectedTime.After(monitor.lastConnectedTime) {
			// minMonitor > monitor
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

func getComputeChassis() (string, error) {
	const chassisTypeFilePath = "/sys/class/dmi/id/chassis_type"
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	hostnameObj := hostname1.NewHostname(systemBus)
	chassis, err := hostnameObj.Chassis().Get(0)
	if err != nil {
		return "", err
	}
	if chassis == "" || chassis == "desktop" {
		chassisNum, err := ioutil.ReadFile(chassisTypeFilePath)
		if err != nil {
			logger.Warning(err)
			return "", err
		}
		switch string(bytes.TrimSpace(chassisNum)) {
		case "13":
			chassis = "all-in-one"
		}
	}
	return chassis, nil
}

func getGraphicsCardPciId() string {
	var pciId string
	subsystems := []string{"drm"}
	gudevClient := gudev.NewClient(subsystems)
	if gudevClient == nil {
		return ""
	}
	defer gudevClient.Unref()

	devices := gudevClient.QueryBySubsystem("drm")
	defer func() {
		for _, dev := range devices {
			dev.Unref()
		}
	}()

	for _, dev := range devices {
		name := dev.GetName()
		if strings.HasPrefix(name, "card") && strings.Contains(name, "-") {
			if dev.GetSysfsAttr("status") == "connected" {
				cardDevice := dev.GetParent()
				parentDevice := cardDevice.GetParent()
				pciId = parentDevice.GetProperty("PCI_ID")
				cardDevice.Unref()
				parentDevice.Unref()
				break
			}
		}
	}

	return pciId
}

func getFilterRefreshRateMap(pciId string) map[string]string {
	var filterRefreshRateMap map[string]string
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(filterFilePath)
	if err != nil {
		logger.Warning("failed to load filter.conf, error:", err)
		return nil
	}

	filterRefreshRateMap, err = kf.GetSection(strings.ToLower(pciId))
	if err != nil {
		logger.Warning("failed to get filter refresh rate map, err:", err)
		return nil
	}

	return filterRefreshRateMap
}

func containsRate(src, target string) bool {
	// delete space character in src
	src = strings.Replace(src, " ", "", -1)

	arr := strings.Split(src, ",")
	for _, s := range arr {
		if target == s {
			return true
		}
	}
	return false
}

func filterModeInfosByRefreshRate(modes []ModeInfo) []ModeInfo {
	var reservedModes []ModeInfo

	pciId := getGraphicsCardPciId()
	if pciId == "" {
		logger.Warning("failed to get current using graphics card pci id")
		return modes
	}

	// no refresh rate need to be filtered, directly return
	filterRefreshRateMap := getFilterRefreshRateMap(pciId)
	if len(filterRefreshRateMap) == 0 {
		return modes
	}

	for _, modeInfo := range modes {
		resolution := strconv.FormatUint(uint64(modeInfo.Width), 10) + "*" + strconv.FormatUint(uint64(modeInfo.Height), 10)

		// refresh rates need to be filtered at this resolution
		if value, ok := filterRefreshRateMap[resolution]; ok {
			rate := fmt.Sprintf("%.2f", modeInfo.Rate)
			if containsRate(value, rate) {
				continue
			} else {
				reservedModes = append(reservedModes, modeInfo)
			}
		} else {
			reservedModes = append(reservedModes, modeInfo)
		}
	}
	return reservedModes
}
