package display

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"pkg.deepin.io/dde/startdde/wl_display/ddewloutput"
)

type monitorIdGenerator struct {
	nextId    uint32
	uuidIdMap map[string]uint32
	mu        sync.Mutex
}

func newMonitorIdGenerator() *monitorIdGenerator {
	return &monitorIdGenerator{
		nextId:    1,
		uuidIdMap: make(map[string]uint32),
	}
}

func (mig *monitorIdGenerator) getId(uuid string) uint32 {
	mig.mu.Lock()
	defer mig.mu.Unlock()

	id, ok := mig.uuidIdMap[uuid]
	if ok {
		return id
	}
	id = mig.nextId
	mig.nextId++
	mig.uuidIdMap[uuid] = id
	return id
}

type KOutputInfo struct {
	Uuid         string      `json:"uuid"`
	Edid         string      `json:"edid_base64"`
	Enabled      int32       `json:"enabled"`
	X            int32       `json:"x"`
	Y            int32       `json:"y"`
	Width        int32       `json:"width"`
	Height       int32       `json:"height"`
	RefreshRate  int32       `json:"refresh_rate"`
	Manufacturer string      `json:"manufacturer"`
	Model        string      `json:"model"`
	ModeInfos    []KModeInfo `json:"ModeInfo"`
	PhysHeight   int32       `json:"phys_height"`
	PhysWidth    int32       `json:"phys_width"`
	Transform    int32       `json:"transform"`
	Scale        float64     `json:"scale"`
}

func newKOutputInfoByUUID(uuid string) (*KOutputInfo, error) {
	sinfo, err := ddewloutput.GetScreenInfo()
	if err != nil {
		return nil, err
	}

	info := sinfo.Outputs.Get(uuid)
	if info == nil {
		return nil, fmt.Errorf("not found output by %s", uuid)
	}

	var kinfo = KOutputInfo{
		Uuid:         info.UUID,
		Model:        info.Name,
		Manufacturer: info.Manufacturer,
		X:            info.X,
		Y:            info.Y,
		Width:        info.Width,
		Height:       info.Height,
		RefreshRate:  int32(info.Refresh * 1000),
		PhysWidth:    info.PhysWidth,
		PhysHeight:   info.PhysHeight,
		Transform:    info.Transform,
		Scale:        info.ScaleF,
		ModeInfos:    []KModeInfo{},
	}
	if needSwapWidthHeight(kinfo.rotation()) {
		kinfo.Width, kinfo.Height = kinfo.Height, kinfo.Width
	}

	if info.Enabled {
		kinfo.Enabled = 1
	} else {
		kinfo.Enabled = 0
	}

	for _, m := range info.Modes {
		kinfo.ModeInfos = append(kinfo.ModeInfos, KModeInfo{
			Id:          m.ID,
			Width:       m.Width,
			Height:      m.Height,
			RefreshRate: int32(m.Refresh * 1000),
			Flags:       int32(m.Flag),
		})
	}

	return &kinfo, nil
}

func (m *Manager) applyByWLOutput() error {
	var disabledMonitors []*Monitor
	args := make([]string, 0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	for _, monitor := range m.monitorMap {
		trans := int32(randrRotationToTransform(int(monitor.Rotation)))
		if !monitor.Enabled {
			disabledMonitors = append(disabledMonitors, monitor)
			continue
		}
		logger.Debug("---------Will apply:", monitor.Name, monitor.uuid, monitor.Enabled, monitor.X, monitor.Y, monitor.CurrentMode, trans)

		args = append(args, monitor.uuid, fmt.Sprint("1"),
			fmt.Sprintf("%d", monitor.X), fmt.Sprintf("%d", monitor.Y), fmt.Sprintf("%d", monitor.CurrentMode.Width),
			fmt.Sprintf("%d", monitor.CurrentMode.Height), fmt.Sprintf("%d", int32(monitor.CurrentMode.Rate*1000)),
			fmt.Sprintf("%d", trans))
	}

	if len(args) > 0 {
		cmdline := exec.CommandContext(ctx, "/usr/bin/dde_wloutput", "set")
		cmdline.Args = append(cmdline.Args, args...)
		logger.Info("cmd line args:", cmdline.Args)

		data, err := cmdline.CombinedOutput()
		cancel()
		// ignore timeout signal
		if err != nil && !strings.Contains(err.Error(), "killed") {
			logger.Warningf("%s(%s)", string(data), err)
			return err
		}
		// wait request done
		//time.Sleep(time.Millisecond * 500)
	}

	for _, monitor := range disabledMonitors {
		logger.Debug("-----------Will disable output:", monitor.Name)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		data, err := exec.CommandContext(ctx, "/usr/bin/dde_wloutput", "set", monitor.uuid, "0", "0", "0", "0", "0", "0", "0").CombinedOutput()
		cancel()
		// ignore timeout signal
		if err != nil && !strings.Contains(err.Error(), "killed") {
			logger.Warningf("%s(%s)", string(data), err)
			return err
		}
		// wait request done
		//time.Sleep(time.Millisecond * 500)
	}
	return nil
}

func (oi *KOutputInfo) getModes() (result []ModeInfo) {
	for _, mi := range oi.ModeInfos {
		result = append(result, mi.toModeInfo())
	}
	sort.Sort(sort.Reverse(ModeInfos(result)))
	return
}

const (
	OutputDeviceTransformNormal     = 0
	OutputDeviceTransform90         = 1
	OutputDeviceTransform180        = 2
	OutputDeviceTransform270        = 3
	OutputDeviceTransformFlipped    = 4
	OutputDeviceTransformFlipped90  = 5
	OutputDeviceTransformFlipped180 = 6
	OutputDeviceTransformFlipped270 = 7
)

const (
	OutputDeviceModeCurrent   = 1 << 0
	OutputDeviceModePreferred = 1 << 1
)

func (oi *KOutputInfo) getBestMode() ModeInfo {
	var preferredMode *KModeInfo
	for _, info := range oi.ModeInfos {
		if info.Flags&OutputDeviceModePreferred != 0 {
			preferredMode = &info
			break
		}
	}

	if preferredMode == nil {
		// not found preferred mode
		return getMaxAreaOutputDeviceMode(oi.ModeInfos).toModeInfo()
	}
	return preferredMode.toModeInfo()
}

func (oi *KOutputInfo) getCurrentMode() ModeInfo {
	for _, info := range oi.ModeInfos {
		if info.Flags&OutputDeviceModeCurrent != 0 {
			return info.toModeInfo()
		}
	}
	return ModeInfo{}
}

func (oi *KOutputInfo) rotation() uint16 {
	switch oi.Transform {
	case OutputDeviceTransformNormal:
		return randr.RotationRotate0
	case OutputDeviceTransform90:
		return randr.RotationRotate90
	case OutputDeviceTransform180:
		return randr.RotationRotate180
	case OutputDeviceTransform270:
		return randr.RotationRotate270

	case OutputDeviceTransformFlipped:
		return randr.RotationRotate0
	case OutputDeviceTransformFlipped90:
		return randr.RotationRotate90
	case OutputDeviceTransformFlipped180:
		return randr.RotationRotate180
	case OutputDeviceTransformFlipped270:
		return randr.RotationRotate270
	}
	return 0
}

func randrRotationToTransform(rotation int) int {
	switch rotation {
	case randr.RotationRotate0:
		return OutputDeviceTransformNormal
	case randr.RotationRotate90:
		return OutputDeviceTransform90
	case randr.RotationRotate180:
		return OutputDeviceTransform180
	case randr.RotationRotate270:
		return OutputDeviceTransform270
	}
	return 0
}

func getMaxAreaOutputDeviceMode(modes []KModeInfo) KModeInfo {
	if len(modes) == 0 {
		return KModeInfo{}
	}
	maxAreaMode := modes[0]
	for _, mode := range modes[1:] {
		if int(maxAreaMode.Width)*int(maxAreaMode.Height) < int(mode.Width)*int(mode.Height) {
			maxAreaMode = mode
		}
	}
	return maxAreaMode
}

func (oi *KOutputInfo) getEnabled() bool {
	return int32ToBool(oi.Enabled)
}

func (oi *KOutputInfo) getName() string {
	return getOutputDeviceName(oi.Model, oi.Manufacturer)
}

type KModeInfo struct {
	Id          int32 `json:"id"`
	Width       int32 `json:"width"`
	Height      int32 `json:"height"`
	RefreshRate int32 `json:"refresh_rate"`
	Flags       int32 `json:"flags"`
}

func (mi KModeInfo) toModeInfo() ModeInfo {
	return ModeInfo{
		Id:     uint32(mi.Id),
		name:   mi.name(),
		Width:  uint16(mi.Width),
		Height: uint16(mi.Height),
		Rate:   mi.rate(),
	}
}

func (mi KModeInfo) name() string {
	return fmt.Sprintf("%dx%d", mi.Width, mi.Height)
}

func (mi KModeInfo) rate() float64 {
	return float64(mi.RefreshRate) / 1000.0
}

func unmarshalOutputInfos(str string) ([]*KOutputInfo, error) {
	var v outputInfoWrap
	err := json.Unmarshal([]byte(str), &v)
	if err != nil {
		return nil, err
	}
	return v.OutputInfo, nil
}

func unmarshalOutputInfo(str string) (*KOutputInfo, error) {
	var v outputInfoWrap
	err := json.Unmarshal([]byte(str), &v)
	if err != nil {
		return nil, err
	}
	if len(v.OutputInfo) == 0 {
		return nil, errors.New("length of slice v.OutputInfo is 0")
	}
	return v.OutputInfo[0], nil
}

type outputInfoWrap struct {
	OutputInfo []*KOutputInfo
}

func (m *Manager) listOutput() ([]*KOutputInfo, error) {
	var outputJ string
	var duration = time.Millisecond * 500
	// sometimes got the output list will return nil, this is the output service not inited yet.
	// so try got 3 times.
	for i := 0; i < 3; i++ {
		data, err := m.management.ListOutput(0)
		if len(data) != 0 {
			outputJ = data
			break
		}

		if err != nil || len(data) == 0 {
			logger.Warning("Failed to get output list:", err)
		}
		time.Sleep(duration)
		duration += 100
	}
	logger.Debug("outputJ:", outputJ)
	return unmarshalOutputInfos(outputJ)
}

// such as: make('dell'), model('eDP-1-dell'), so name is 'eDP-1'
func getOutputDeviceName(model, make string) string {
	logger.Debugf("[DEBUG] get name: '%s', '%s'", model, make)
	name := getNameFromModelAndMake(model, make)
	if name != model {
		return name
	}
	names := strings.Split(model, "-")
	if len(names) <= 2 {
		return getNameFromModel(model)
	}

	idx := len(names) - 1
	for ; idx > 1; idx-- {
		if len(names[idx]) > 1 {
			continue
		}
		break
	}
	return strings.Join(names[:idx+1], "-")
}

func getNameFromModel(model string) string {
	idx := strings.IndexByte(model, ' ')
	if idx == -1 {
		return model
	}
	return model[:idx]
}

func getNameFromModelAndMake(model, make string) string {
	preMake := strings.Split(make, " ")[0]
	name := strings.Split(model, preMake)[0]
	return strings.TrimRight(name, "-")
}

func int32ToBool(v int32) bool {
	return v != 0
}
