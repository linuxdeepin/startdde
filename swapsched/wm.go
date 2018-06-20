package swapsched

import (
	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
)

type ActiveWindowHandler func(int, int)

func (cb ActiveWindowHandler) Monitor() error {
	conn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		return err
	}

	root := conn.GetDefaultScreen().Root
	err = x.ChangeWindowAttributesChecked(conn, root, x.CWEventMask, []uint32{
		x.EventMaskPropertyChange}).Check(conn)
	if err != nil {
		logger.Warning(err)
		return err
	}

	atomNetActiveWindow, err := conn.GetAtom("_NET_ACTIVE_WINDOW")
	if err != nil {
		logger.Warning("failed to get _NET_ACTIVE_WINDOW atom:", err)
		return err
	}

	eventChan := make(chan x.GenericEvent, 10)
	conn.AddEventChan(eventChan)

	handlePropNotifyEvent := func(event *x.PropertyNotifyEvent) {
		if event.Atom != atomNetActiveWindow || event.Window != root {
			return
		}

		activeWin, err := ewmh.GetActiveWindow(conn).Reply(conn)
		if err != nil {
			logger.Warning(err)
			return
		}

		if activeWin != 0 {
			pid, err := ewmh.GetWMPid(conn, activeWin).Reply(conn)
			if err != nil {
				logger.Warning(err)
				return
			}
			if pid != 0 && cb != nil {
				cb(int(pid), int(activeWin))
			}
		}
	}

	for ev := range eventChan {
		switch ev.GetEventCode() {
		case x.PropertyNotifyEventCode:
			event, _ := x.NewPropertyNotifyEvent(ev)
			handlePropNotifyEvent(event)
		}
	}
	return nil
}

func (d *Dispatcher) ActiveWindowHandler(pid int, xid int) {
	// pid != 0
	d.Lock()
	defer d.Unlock()

	if pid == 0 {
		d.setActiveApp(nil)
		// unset active app but don't do balance now.
		return
	}

	if d.activeXID == xid {
		return
	}
	d.activeXID = xid

	if d.activeApp != nil && d.activeApp.HasChild(pid) {
		return
	}

	var newActive *UIApp
	for _, app := range d.inactiveApps {
		if app.HasChild(pid) {
			newActive = app
			break
		}
	}
	d.setActiveApp(newActive)
	d.balance()
}
