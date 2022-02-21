package display

import (
	"encoding/json"
	"path"

	"github.com/godbus/dbus"
	kwin "github.com/linuxdeepin/go-dbus-factory/org.kde.kwin"
	"golang.org/x/xerrors"
)

type wlKwinAssociation struct {
	ScreenID    int32  `json:"ScreenId"`
	ScreenUUID  string `json:"ScreenUuid"`
	TouchDevice string
}

type wlTouchscreenManager struct {
	baseTouchscreenManager

	bus               *dbus.Conn
	kwin              kwin.KWin
	nextTouchscreenID int32
	associationInfo   []wlKwinAssociation
}

func newWaylandTouchscreenManager(bus *dbus.Conn, sysBus *dbus.Conn) *wlTouchscreenManager {
	tm := new(wlTouchscreenManager)
	tm.baseTouchscreenManager.sysBus = sysBus
	tm.baseTouchscreenManager.outer = tm
	tm.bus = bus
	tm.kwin = kwin.NewKWin(bus)
	tm.nextTouchscreenID = 1
	tm.associationInfo = make([]wlKwinAssociation, 0, 0)

	return tm
}

func (tm *wlTouchscreenManager) refreshDevicesFromDisplayServer() {
	assInfo, err := tm.kwin.GetTouchDeviceToScreenInfo(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	err = json.Unmarshal([]byte(assInfo), &tm.associationInfo)
	if err != nil {
		logger.Warning(err)
	}
}

func (tm *wlTouchscreenManager) associateTouchscreen(monitor *Monitor, touchUUID string) error {
	var screenID int32 = -1
	for _, i := range tm.associationInfo {
		if i.ScreenUUID == monitor.uuidV0 {
			screenID = i.ScreenID
		}
	}
	if screenID == -1 {
		return xerrors.New("monitor uuid not found")
	}

	for _, touch := range tm.list {
		if touch.UUID != touchUUID {
			continue
		}

		touchDeviceSysName := path.Base(touch.DeviceNode)

		err := tm.kwin.SetTouchDeviceToScreenId(0, touchDeviceSysName, screenID)
		if err != nil {
			logger.Warning(err)
		}
	}

	return nil
}

func (tm *wlTouchscreenManager) completeTouchscreenID(t *Touchscreen) {
	t.Id = tm.nextTouchscreenID
	tm.nextTouchscreenID++
}
