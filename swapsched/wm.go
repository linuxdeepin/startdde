package swapsched

import (
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xevent"
	"github.com/BurntSushi/xgbutil/xprop"
	"github.com/BurntSushi/xgbutil/xwindow"
)

type ActiveWindowHandler func(int, int)

func (cb ActiveWindowHandler) Monitor() error {
	xu, err := xgbutil.NewConn()
	if err != nil {
		return err
	}
	root := xu.RootWin()
	xwindow.New(xu, root).Listen(xproto.EventMaskPropertyChange)

	AtomNetActiveWindow, err := xprop.Atm(xu, "_NET_ACTIVE_WINDOW")
	if err != nil {
		logger.Warning("failed to get _NET_ACTIVE_WINDOW atom")
		return err
	}

	xevent.PropertyNotifyFun(
		func(X *xgbutil.XUtil, e xevent.PropertyNotifyEvent) {
			if e.Atom != AtomNetActiveWindow {
				return
			}

			activeWin, err := xprop.PropValWindow(xprop.GetProperty(xu, root, "_NET_ACTIVE_WINDOW"))
			if err != nil {
				logger.Warning(err)
				return
			}
			if activeWin != 0 {
				pid, err := xprop.PropValNum(xprop.GetProperty(xu, activeWin, "_NET_WM_PID"))
				if err != nil {
					logger.Warningf("failed to get pid for window %d: %v", activeWin, err)
					return
				}
				if pid != 0 && cb != nil {
					cb(int(pid), int(activeWin))
				}
			}
		}).Connect(xu, root)
	xevent.Main(xu)
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
