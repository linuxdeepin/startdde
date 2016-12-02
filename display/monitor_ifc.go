package display

import (
	"fmt"
)

func (m *MonitorInfo) Enable(enabled bool) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doEnable(enabled)
	if err != nil {
		logger.Warningf("Toggle '%s' to '%v' failed: %v", m.Name, enabled, err)
		return err
	}
	m.cfg.Enabled = enabled
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetMode(v uint32) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	// TODO: Why needs to modify monitor properties? Try only modify cfg properties.
	err := m.doSetMode(v)
	if err != nil {
		logger.Warningf("set mode '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Width, m.cfg.Height, m.cfg.RefreshRate = m.CurrentMode.Width, m.CurrentMode.Height, m.CurrentMode.Rate
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetModeBySize(w, h uint16) error {
	mode := m.Modes.QueryBySize(w, h)
	if mode.Id == 0 {
		logger.Warning("Invalid mode size:", w, h)
		return fmt.Errorf("The mode size %dx%d invalid", w, h)
	}

	return m.SetMode(mode.Id)
}

func (m *MonitorInfo) SetPosition(x, y int16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	m.doSetPosition(x, y)
	m.cfg.X, m.cfg.Y = x, y
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetRotation(v uint16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doSetRotation(v)
	if err != nil {
		logger.Warningf("Set rotation '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Rotation = v
	_dpy.detectHasChanged()
	return nil
}

func (m *MonitorInfo) SetReflect(v uint16) error {
	m.locker.Lock()
	defer m.locker.Unlock()
	err := m.doSetReflect(v)
	if err != nil {
		logger.Warningf("Set reflect '%v' for '%s' failed: %v", v, m.Name, err)
		return err
	}
	m.cfg.Reflect = v
	_dpy.detectHasChanged()
	return nil
}
