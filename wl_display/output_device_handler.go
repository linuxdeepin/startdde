package display

import (
	"fmt"
	"strings"

	"pkg.deepin.io/dde/startdde/wl_display/org_kde_kwin/outputdevice"

	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

type outputDeviceHandler struct {
	core    *outputdevice.Outputdevice
	regName uint32
	id      uint32

	// mode
	modes         []outputDeviceMode
	modesTmp      []outputDeviceMode
	currentMode   outputDeviceMode
	preferredMode outputDeviceMode

	// geometry
	x              int32
	y              int32
	physicalWidth  int32
	physicalHeight int32
	make           string
	model          string
	transform      int32

	uuid    string
	edid    string
	eisaId  string
	enabled bool

	doneCb    func(odh *outputDeviceHandler)
	enabledCb func(odh *outputDeviceHandler)
}

func (odh *outputDeviceHandler) name() string {
	return getNameFromModel(odh.model)
}

func getNameFromModel(model string) string {
	idx := strings.IndexByte(model, ' ')
	if idx == -1 {
		return model
	}
	return model[:idx]
}

func (odh *outputDeviceHandler) rotation() uint16 {
	switch odh.transform {
	case outputdevice.OutputdeviceTransformNormal:
		return randr.RotationRotate0
	case outputdevice.OutputdeviceTransform90:
		return randr.RotationRotate90
	case outputdevice.OutputdeviceTransform180:
		return randr.RotationRotate180
	case outputdevice.OutputdeviceTransform270:
		return randr.RotationRotate270

	case outputdevice.OutputdeviceTransformFlipped:
		return randr.RotationRotate0
	case outputdevice.OutputdeviceTransformFlipped90:
		return randr.RotationRotate90
	case outputdevice.OutputdeviceTransformFlipped180:
		return randr.RotationRotate180
	case outputdevice.OutputdeviceTransformFlipped270:
		return randr.RotationRotate270
	}
	return 0
}

func toOutputDeviceTransform(rotation uint16, reflect uint16) int32 {
	// 暂时不管 reflect
	switch rotation {
	case randr.RotationRotate0:
		return outputdevice.OutputdeviceTransformNormal
	case randr.RotationRotate90:
		return outputdevice.OutputdeviceTransform90
	case randr.RotationRotate180:
		return outputdevice.OutputdeviceTransform180
	case randr.RotationRotate270:
		return outputdevice.OutputdeviceTransform270
	default:
		logger.Warning("invalid rotation", rotation)
		return outputdevice.OutputdeviceTransformNormal
	}
}

func (odh *outputDeviceHandler) getModes() []ModeInfo {
	result := make([]ModeInfo, len(odh.modes))
	for i, mode := range odh.modes {
		result[i] = mode.toModeInfo()
	}
	return result
}

func (odh *outputDeviceHandler) getBestMode() ModeInfo {
	if odh.preferredMode.Width == -1 {
		// not found preferred mode
		return getMaxAreaOutputDeviceMode(odh.modes).toModeInfo()
	}
	return odh.preferredMode.toModeInfo()
}

func getMaxAreaOutputDeviceMode(modes []outputDeviceMode) outputDeviceMode {
	if len(modes) == 0 {
		return outputDeviceMode{}
	}
	maxAreaMode := modes[0]
	for _, mode := range modes[1:] {
		if int(maxAreaMode.Width)*int(maxAreaMode.Height) < int(mode.Width)*int(mode.Height) {
			maxAreaMode = mode
		}
	}
	return maxAreaMode
}

func (odh *outputDeviceHandler) getCurrentMode() ModeInfo {
	return odh.currentMode.toModeInfo()
}

func newOutputDeviceHandler(device *outputdevice.Outputdevice, regName uint32) *outputDeviceHandler {
	odh := &outputDeviceHandler{core: device}
	odh.id = uint32(device.Id())
	odh.regName = regName
	odh.preferredMode.Width = -1

	device.AddModeHandler(odh)
	device.AddDoneHandler(odh)
	device.AddEdidHandler(odh)
	device.AddGeometryHandler(odh)
	device.AddEnabledHandler(odh)
	device.AddEisaIdHandler(odh)
	device.AddUuidHandler(odh)
	return odh
}

func (odh *outputDeviceHandler) remove() {
	odh.core.RemoveModeHandler(odh)
	odh.core.RemoveDoneHandler(odh)
	odh.core.RemoveEdidHandler(odh)
	odh.core.RemoveGeometryHandler(odh)
	odh.core.RemoveEnabledHandler(odh)
	odh.core.RemoveEisaIdHandler(odh)
	odh.core.RemoveUuidHandler(odh)
}

func (odh *outputDeviceHandler) HandleOutputdeviceMode(ev outputdevice.OutputdeviceModeEvent) {
	logger.Debugf("output device mode device id: %d, ev: %#v", odh.id, ev)
	mode := toOutputDeviceMode(ev)
	odh.modesTmp = append(odh.modesTmp, mode)
	if ev.Flags&outputdevice.OutputdeviceModeCurrent != 0 {
		odh.currentMode = mode
	}
	if ev.Flags&outputdevice.OutputdeviceModePreferred != 0 {
		odh.preferredMode = mode
	}
}

type outputDeviceMode struct {
	Flags   uint32
	Width   int32
	Height  int32
	Refresh int32
	ModeId  int32
}

func (mode outputDeviceMode) toModeInfo() ModeInfo {
	return ModeInfo{
		Id:     uint32(mode.ModeId),
		name:   mode.name(),
		Width:  uint16(mode.Width),
		Height: uint16(mode.Height),
		Rate:   mode.rate(),
	}
}

func (m outputDeviceMode) name() string {
	return fmt.Sprintf("%dx%d", m.Width, m.Height)
}

func (m outputDeviceMode) rate() float64 {
	return float64(m.Refresh) / 1000.0
}

func toOutputDeviceMode(ev outputdevice.OutputdeviceModeEvent) outputDeviceMode {
	return outputDeviceMode{
		Flags:   ev.Flags,
		Width:   ev.Width,
		Height:  ev.Height,
		Refresh: ev.Refresh,
		ModeId:  ev.ModeId,
	}
}

func (odh *outputDeviceHandler) HandleOutputdeviceDone(ev outputdevice.OutputdeviceDoneEvent) {
	if len(odh.modesTmp) == 0 ||
		(len(odh.modesTmp) == 1 && len(odh.modes) > 0) {
		// do not update odh.modes
	} else {
		odh.modes = odh.modesTmp
		odh.dump()
	}
	odh.modesTmp = nil
	if odh.doneCb != nil {
		odh.doneCb(odh)
	}
}

func (odh *outputDeviceHandler) dump() {
	logger.Debug("output device id:", odh.id)
	logger.Debug("output device model:", odh.model)
	for _, mode := range odh.modes {
		logger.Debugf("%#v", mode)
	}
}

func (odh *outputDeviceHandler) HandleOutputdeviceGeometry(ev outputdevice.OutputdeviceGeometryEvent) {
	logger.Debugf("output device geometry: %#v", ev)
	odh.x = ev.X
	odh.y = ev.Y
	odh.physicalWidth = ev.PhysicalWidth
	odh.physicalHeight = ev.PhysicalHeight
	odh.make = ev.Make
	odh.model = ev.Model
	odh.transform = ev.Transform
}

func (odh *outputDeviceHandler) HandleOutputdeviceEdid(ev outputdevice.OutputdeviceEdidEvent) {
	logger.Debugf("output device edid %d %q", odh.id, ev.Raw)
	odh.edid = ev.Raw
}

func (odh *outputDeviceHandler) HandleOutputdeviceEisaId(ev outputdevice.OutputdeviceEisaIdEvent) {
	logger.Debugf("output device eisaId %d %q", odh.id, ev.EisaId)
	odh.eisaId = ev.EisaId
}

func (odh *outputDeviceHandler) HandleOutputdeviceEnabled(ev outputdevice.OutputdeviceEnabledEvent) {
	logger.Debugf("output device enabled %d %v", odh.id, ev.Enabled)
	odh.enabled = int32ToBool(ev.Enabled)
	if odh.enabledCb != nil {
		odh.enabledCb(odh)
	}
}

func (odh *outputDeviceHandler) HandleOutputdeviceUuid(ev outputdevice.OutputdeviceUuidEvent) {
	logger.Debugf("output device uuid %d %v", odh.id, ev.Uuid)
	odh.uuid = ev.Uuid
}

func int32ToBool(v int32) bool {
	return v != 0
}
