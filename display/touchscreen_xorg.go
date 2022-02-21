package display

import (
	"fmt"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-api/dxinput"
	"github.com/linuxdeepin/dde-api/dxinput/common"
	dxutils "github.com/linuxdeepin/dde-api/dxinput/utils"
	dgesture "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.gesture"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

const (
	busTypeUnknown uint8 = iota
	busTypeUSB
)

const (
	rotationReflectAll = randr.RotationReflectX | randr.RotationReflectY
)

var (
	_devInfos common.DeviceInfos
)

type xTouchscreenManager struct {
	baseTouchscreenManager

	list dxTouchscreens
}

func newXTouchscreenManager(sysBus *dbus.Conn) *xTouchscreenManager {
	tm := new(xTouchscreenManager)
	tm.baseTouchscreenManager.sysBus = sysBus
	tm.baseTouchscreenManager.outer = tm

	return tm
}

func (tm *xTouchscreenManager) refreshDevicesFromDisplayServer() {
	tm.getDeviceInfos(true)
}

func (tm *xTouchscreenManager) associateTouchscreen(monitor *Monitor, touchUUID string) error {
	touchIDs := make([]int32, 0)
	for _, touchscreen := range tm.list {
		if touchscreen.UUID != touchUUID {
			continue
		}

		touchIDs = append(touchIDs, touchscreen.Id)
	}
	if len(touchIDs) == 0 {
		return fmt.Errorf("invalid touchscreen: %s", touchUUID)
	}

	ignoreGestureFunc := func(id int32, ignore bool) {
		hasNode := dxutils.IsPropertyExist(id, "Device Node")
		if hasNode {
			data, item := dxutils.GetProperty(id, "Device Node")
			node := string(data[:item])

			gestureObj := dgesture.NewGesture(tm.sysBus)
			err := gestureObj.SetInputIgnore(0, node, ignore)
			if err != nil {
				logger.Warning(err)
			}
		}
	}

	if monitor.Enabled {
		matrix := genTransformationMatrix(monitor.X, monitor.Y, monitor.Width, monitor.Height, monitor.Rotation|monitor.Reflect)

		for _, touchID := range touchIDs {
			dxTouchscreen, err := dxinput.NewTouchscreen(touchID)
			if err != nil {
				logger.Warning(err)
				continue
			}
			logger.Debugf("matrix: %v, touchscreen: %s(%d)", matrix, touchUUID, touchID)

			err = dxTouchscreen.Enable(true)
			if err != nil {
				logger.Warning(err)
				continue
			}
			ignoreGestureFunc(dxTouchscreen.Id, false)

			err = dxTouchscreen.SetTransformationMatrix(matrix)
			if err != nil {
				logger.Warning(err)
				continue
			}
		}
	} else {
		for _, touchID := range touchIDs {
			dxTouchscreen, err := dxinput.NewTouchscreen(touchID)
			if err != nil {
				logger.Warning(err)
				continue
			}
			logger.Debugf("touchscreen %s(%d) disabled", touchUUID, touchID)
			ignoreGestureFunc(dxTouchscreen.Id, true)
			err = dxTouchscreen.Enable(false)
			if err != nil {
				logger.Warning(err)
				continue
			}
		}
	}

	return nil
}

func (tm *xTouchscreenManager) completeTouchscreenID(t *Touchscreen) {
	for _, v := range tm.getDeviceInfos(false) {
		if v.Type != common.DevTypeTouchscreen {
			continue
		}

		tmp, _ := dxinput.NewTouchscreenFromDevInfo(v)
		data, num := dxutils.GetProperty(tmp.Id, "Device Node")
		if len(data) == 0 {
			logger.Warningf("could not get DeviceNode for %s (%d)", tmp.Name, tmp.Id)
			continue
		}

		deviceNode := string(data[:num])
		logger.Warningf("deviceNode: %s", deviceNode)

		logger.Warningf("devNode: %s, deviceNode: %s", t.DeviceNode, deviceNode)
		if t.DeviceNode != deviceNode {
			continue
		}

		t.Id = tmp.Id
		break
	}
}

func (tm *xTouchscreenManager) getDeviceInfos(force bool) common.DeviceInfos {
	if force || len(_devInfos) == 0 {
		_devInfos = dxutils.ListDevice()
	}

	return _devInfos
}

type transformationMatrix [9]float32

func (m *transformationMatrix) set(row int, col int, v float32) {
	m[row*3+col] = v
}

func (m *transformationMatrix) setUnity() {
	m.set(0, 0, 1)
	m.set(1, 1, 1)
	m.set(2, 2, 1)
}

func (m *transformationMatrix) s4(x02 float32, x12 float32, d1 float32, d2 float32, mainDiag bool) {
	m.set(0, 2, x02)
	m.set(1, 2, x12)

	if mainDiag {
		m.set(0, 0, d1)
		m.set(1, 1, d2)
	} else {
		m.set(0, 0, 0)
		m.set(1, 1, 0)
		m.set(0, 1, d1)
		m.set(1, 0, d2)
	}
}

func genTransformationMatrix(offsetX int16, offsetY int16,
	screenWidth uint16, screenHeight uint16,
	rotation uint16) transformationMatrix {

	// 必须新的 X 链接才能获取最新的 WidthInPixels 和 HeightInPixels
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning("failed to connect to x server")
		return genTransformationMatrixAux(offsetX, offsetY, screenWidth, screenHeight, screenWidth, screenHeight, rotation)
	}

	// total display size
	width := xConn.GetDefaultScreen().WidthInPixels
	height := xConn.GetDefaultScreen().HeightInPixels
	xConn.Close()

	return genTransformationMatrixAux(offsetX, offsetY, screenWidth, screenHeight, width, height, rotation)
}

func genTransformationMatrixAux(offsetX int16, offsetY int16,
	screenWidth uint16, screenHeight uint16,
	totalDisplayWidth uint16, totalDisplayHeight uint16,
	rotation uint16) transformationMatrix {

	var matrix transformationMatrix
	matrix.setUnity()

	x := float32(offsetX) / float32(totalDisplayWidth)
	y := float32(offsetY) / float32(totalDisplayHeight)

	w := float32(screenWidth) / float32(totalDisplayWidth)
	h := float32(screenHeight) / float32(totalDisplayHeight)

	/*
	 * There are 16 cases:
	 * Rotation X Reflection
	 * Rotation: 0 | 90 | 180 | 270
	 * Reflection: None | X | Y | XY
	 *
	 * They are spelled out instead of doing matrix multiplication to avoid
	 * any floating point errors.
	 */
	switch int(rotation) {
	case randr.RotationRotate0:
		fallthrough
	case randr.RotationRotate180 | rotationReflectAll:
		matrix.s4(x, y, w, h, true)

	case randr.RotationReflectX | randr.RotationRotate0:
		fallthrough
	case randr.RotationReflectY | randr.RotationRotate180:
		matrix.s4(x+w, y, -w, h, true)

	case randr.RotationReflectY | randr.RotationRotate0:
		fallthrough
	case randr.RotationReflectX | randr.RotationRotate180:
		matrix.s4(x, y+h, w, -h, true)

	case randr.RotationRotate90:
		fallthrough
	case randr.RotationRotate270 | rotationReflectAll: /* left limited - correct in working zone. */
		matrix.s4(x+w, y, -w, h, false)

	case randr.RotationRotate270:
		fallthrough
	case randr.RotationRotate90 | rotationReflectAll: /* left limited - correct in working zone. */
		matrix.s4(x, y+h, w, -h, false)

	case randr.RotationRotate90 | randr.RotationReflectX: /* left limited - correct in working zone. */
		fallthrough
	case randr.RotationRotate270 | randr.RotationReflectY: /* left limited - correct in working zone. */
		matrix.s4(x, y, w, h, false)

	case randr.RotationRotate90 | randr.RotationReflectY: /* right limited - correct in working zone. */
		fallthrough
	case randr.RotationRotate270 | randr.RotationReflectX: /* right limited - correct in working zone. */
		matrix.s4(x+w, y+h, -w, -h, false)

	case randr.RotationRotate180:
		fallthrough
	case rotationReflectAll | randr.RotationRotate0:
		matrix.s4(x+w, y+h, -w, -h, true)
	}

	return matrix
}
