/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package xsettings

import (
	"fmt"
	"os"
	"sync"

	dbus "github.com/godbus/dbus"
	ddeSysDaemon "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
	greeter "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.greeter"
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/log"
)

const (
	xsSchema           = "com.deepin.xsettings"
	defaultScaleFactor = 1.0

	xsDBusPath = "/com/deepin/XSettings"
	xsDBusIFC  = "com.deepin.XSettings"
)

var logger *log.Logger

// XSManager xsettings manager
type XSManager struct {
	service *dbusutil.Service
	conn    *x.Conn
	owner   x.Window

	gs        *gio.Settings
	greeter   *greeter.Greeter
	sysDaemon *ddeSysDaemon.Daemon

	plymouthScalingMu    sync.Mutex
	plymouthScalingTasks []int
	plymouthScaling      bool

	restartOSD bool // whether to restart dde-osd

	// locker for xsettings prop read and write
	settingsLocker sync.RWMutex

	//nolint
	signals *struct {
		SetScaleFactorStarted, SetScaleFactorDone struct{}
	}

	//nolint
	methods *struct {
		ListProps func() `out:"props"`

		GetInteger func() `in:"prop" out:"result"`
		SetInteger func() `in:"prop,v"`

		GetString func() `in:"prop" out:"result"`
		SetString func() `in:"prop,v"`

		GetColor func() `in:"prop" out:"result"`
		SetColor func() `in:"prop,v"`

		GetScaleFactor func() `out:"scale"`
		SetScaleFactor func() `in:"scale"`

		GetScreenScaleFactors func() `out:"factors"`
		SetScreenScaleFactors func() `in:"factors"`
	}
}

type xsSetting struct {
	sType uint8
	prop  string
	value interface{} // int32, string, [4]uint16
}

func NewXSManager(conn *x.Conn, recommendedScaleFactor float64, service *dbusutil.Service) (*XSManager, error) {
	var m = &XSManager{
		conn:    conn,
		service: service,
		gs:      _gs,
	}

	var err error
	m.owner, err = createSettingWindow(m.conn)
	if err != nil {
		return nil, err
	}
	logger.Debug("owner:", m.owner)

	if !isSelectionOwned(settingPropScreen, m.owner, m.conn) {
		logger.Errorf("Owned '%s' failed", settingPropSettings)
		return nil, fmt.Errorf("Owned '%s' failed", settingPropSettings)
	}

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	m.greeter = greeter.NewGreeter(systemBus)
	m.sysDaemon = ddeSysDaemon.NewDaemon(systemBus)

	m.adjustScaleFactor(recommendedScaleFactor)
	err = m.setSettings(m.getSettingsInSchema())
	if err != nil {
		logger.Warning("Change xsettings property failed:", err)
	}

	return m, nil
}

func (m *XSManager) GetInterfaceName() string {
	return xsDBusIFC
}

var _gs *gio.Settings

func GetScaleFactor() float64 {
	return getScaleFactor()
}

func getScaleFactor() float64 {
	scale := _gs.GetDouble(gsKeyScaleFactor)
	return scale
}

func (m *XSManager) adjustScaleFactor(recommendedScaleFactor float64) {
	logger.Debug("recommended scale factor:", recommendedScaleFactor)
	var err error
	if m.gs.GetUserValue(gsKeyScaleFactor) == nil &&
		recommendedScaleFactor != defaultScaleFactor {
		err = m.setScaleFactorWithoutNotify(recommendedScaleFactor)
		if err != nil {
			logger.Warning("failed to set scale factor:", err)
		}
		m.restartOSD = true
	}

	// migrate old configuration
	if os.Getenv("STARTDDE_MIGRATE_SCALE_FACTOR") != "" {
		scaleFactor := getScaleFactor()
		err = m.setScreenScaleFactorsForQt(map[string]float64{"": scaleFactor})
		if err != nil {
			logger.Warning("failed to set scale factor for qt:", err)
		}

		err = cleanUpDdeEnv()
		if err != nil {
			logger.Warning("failed to clean up dde env:", err)
		}
		return
	}

	_, err = os.Stat("/etc/lightdm/deepin/qt-theme.ini")
	if err != nil {
		if os.IsNotExist(err) {
			// lightdm-deepin-greeter does not have the qt-theme.ini file yet.
			scaleFactor := getScaleFactor()
			if scaleFactor != defaultScaleFactor {
				err = m.setScreenScaleFactorsForQt(map[string]float64{"": scaleFactor})
				if err != nil {
					logger.Warning("failed to set scale factor for qt:", err)
				}
			}
		} else {
			logger.Warning(err)
		}
	}
}

func (m *XSManager) setSettings(settings []xsSetting) error {
	m.settingsLocker.Lock()
	defer m.settingsLocker.Unlock()
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return err
	}

	xsInfo := unmarshalSettingData(datas)
	xsInfo.serial++ // auto increment
	for _, s := range settings {
		item := xsInfo.getPropItem(s.prop)
		if item != nil {
			xsInfo.items = xsInfo.modifyProperty(s)
			continue
		}

		if s.value == nil {
			continue
		}

		var tmp *xsItemInfo
		switch s.sType {
		case settingTypeInteger:
			tmp = newXSItemInteger(s.prop, s.value.(int32))
		case settingTypeString:
			tmp = newXSItemString(s.prop, s.value.(string))
		case settingTypeColor:
			tmp = newXSItemColor(s.prop, s.value.([4]uint16))
		}

		xsInfo.items = append(xsInfo.items, *tmp)
		xsInfo.numSettings++
	}

	data := marshalSettingData(xsInfo)
	return changeSettingProp(m.owner, data, m.conn)
}

func (m *XSManager) getSettingsInSchema() []xsSetting {
	var settings []xsSetting
	for _, key := range m.gs.ListKeys() {
		info := gsInfos.getByGSKey(key)
		if info == nil {
			continue
		}

		value, err := info.getValue(m.gs)
		if err != nil {
			logger.Warning(err)
			continue
		}

		settings = append(settings, xsSetting{
			sType: info.getKeySType(),
			prop:  info.xsKey,
			value: value,
		})
	}
	return settings
}

func (m *XSManager) handleGSettingsChanged() {
	gsettings.ConnectChanged(xsSchema, "*", func(key string) {
		switch key {
		case "xft-dpi":
			return
		case gsKeyScaleFactor:
			m.updateDPI()
			return
		case "gtk-cursor-theme-name":
			updateXResources(xresourceInfos{
				&xresourceInfo{
					key:   "Xcursor.theme",
					value: m.gs.GetString("gtk-cursor-theme-name"),
				},
			})
		case gsKeyGtkCursorThemeSize:
			updateXResources(xresourceInfos{
				&xresourceInfo{
					key:   "Xcursor.size",
					value: fmt.Sprintf("%d", m.gs.GetInt(gsKeyGtkCursorThemeSize)),
				},
			})
		case gsKeyWindowScale:
			m.updateDPI()
			return
		}

		info := gsInfos.getByGSKey(key)
		if info == nil {
			return
		}

		value, err := info.getValue(m.gs)
		if err == nil {
			err = m.setSettings([]xsSetting{
				{
					sType: info.getKeySType(),
					prop:  info.xsKey,
					value: value,
				},
			})
			if err != nil {
				logger.Warning(err)
			}
		} else {
			logger.Warning(err)
		}
	})
}

// Start load xsettings module
func Start(conn *x.Conn, l *log.Logger, recommendedScaleFactor float64, service *dbusutil.Service) (*XSManager, error) {
	_gs = gio.NewSettings(xsSchema)
	logger = l
	m, err := NewXSManager(conn, recommendedScaleFactor, service)
	if err != nil {
		logger.Error("Start xsettings failed:", err)
		return nil, err
	}
	m.updateDPI()
	m.updateXResources()
	go m.updateFirefoxDPI()

	err = service.Export(xsDBusPath, m)
	if err != nil {
		logger.Warning("export XSManager failed:", err)
		return nil, err
	}

	m.handleGSettingsChanged()
	return m, nil
}

func (m *XSManager) NeedRestartOSD() bool {
	if m == nil {
		return false
	}
	return m.restartOSD
}
