package wm_kwin

import (
	"time"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.osd"
	"github.com/linuxdeepin/go-dbus-factory/org.kde.kwin"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

const (
	swDBusDest = "com.deepin.WMSwitcher"
	swDBusPath = "/com/deepin/WMSwitcher"
	swDBusIFC  = swDBusDest

	wmName3D = "deepin wm"
	wmName2D = "deepin metacity"

	osdSwitch2DWM    = "SwitchWM2D"
	osdSwitch3DWM    = "SwitchWM3D"
	osdSwitchWMError = "SwitchWMError"
)

var logger *log.Logger

func Start(l *log.Logger, wmChooseLaunched bool) error {
	logger = l

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	service := dbusutil.NewService(sessionBus)
	s := newSwitcher(service)
	err = service.Export(swDBusPath, s)
	if err != nil {
		return err
	}

	err = service.RequestName(swDBusDest)
	if err != nil {
		return err
	}

	s.listenDBusSignal()
	return nil
}

func newSwitcher(service *dbusutil.Service) *Switcher {
	s := &Switcher{
		service: service,
	}

	sigLoop := dbusutil.NewSignalLoop(service.Conn(), 10)
	sigLoop.Start()
	s.compositor = kwin.NewCompositor(service.Conn())
	s.compositor.InitSignalExt(sigLoop, true)
	return s
}

type Switcher struct {
	service                 *dbusutil.Service
	sigLoop                 *dbusutil.SignalLoop
	compositingToggledCount int

	compositor *kwin.Compositor

	signals *struct {
		WMChanged struct {
			wmName string
		}
	}

	methods *struct {
		AllowSwitch func() `out:"allow"`
		CurrentWM   func() `out:"wmName"`
	}
}

func (s *Switcher) AllowSwitch() (bool, *dbus.Error) {
	possible, err := s.compositor.CompositingPossible().Get(0)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return possible, nil
}

func (s *Switcher) CurrentWM() (string, *dbus.Error) {
	active, err := s.compositor.Active().Get(0)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	wmName := wmName2D
	if active {
		wmName = wmName3D
	}
	return wmName, nil
}

func (s *Switcher) RequestSwitchWM() *dbus.Error {
	active, err := s.compositor.Active().Get(0)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if active {
		err = s.compositor.Suspend(0)
	} else {
		err = s.compositor.Resume(0)
	}
	return dbusutil.ToError(err)
}

func (s *Switcher) GetInterfaceName() string {
	return swDBusIFC
}

func (s *Switcher) emitSignalWMChanged(wmName string) {
	err := s.service.Emit(s, "WMChanged", wmName)
	if err != nil {
		logger.Warning(err)
	}
}

func (s *Switcher) listenDBusSignal() {
	_, err := s.compositor.ConnectCompositingToggled(func(active bool) {
		s.compositingToggledCount++
		if s.compositingToggledCount == 1 {
			return
		}

		wmName := wmName2D
		osdName := osdSwitch2DWM

		if active {
			wmName = wmName3D
			osdName = osdSwitch3DWM
		}

		s.emitSignalWMChanged(wmName)
		time.AfterFunc(1*time.Second, func() {
			err := showOSD(osdName)
			if err != nil {
				logger.Warning(err)
			}
		})
	})
	if err != nil {
		logger.Warning(err)
	}
}

func showOSD(name string) error {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	osdObj := osd.NewOSD(sessionBus)
	return osdObj.ShowOSD(0, name)
}
