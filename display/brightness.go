package display

import (
	"encoding/json"
	"fmt"
	"gir/gio-2.0"
	"sync"
)

type brightnessMapManager struct {
	core    map[string]float64
	setting *gio.Settings
	locker  sync.Mutex
}

func (dpy *Display) initBrightnessManager() {
	dpy.brightnessManager = &brightnessMapManager{
		core:    make(map[string]float64),
		setting: dpy.setting,
	}
	err := dpy.brightnessManager.reset()
	if err != nil {
		logger.Warning("Unmarshal failed:", err)
		return
	}
	if len(dpy.brightnessManager.core) == 0 {
		for name, output := range GetDisplayInfo().outputNames {
			if dpy.supportedBacklight(xcon, output) {
				dpy.brightnessManager.set(name,
					dpy.getBacklight(brightnessSetterBacklight))
			} else {
				dpy.brightnessManager.set(name, 1)
			}
		}
	}
	logger.Debug("[initBrightness] result:", dpy.brightnessManager)
}

func (m *brightnessMapManager) set(output string, value float64) {
	m.locker.Lock()
	defer m.locker.Unlock()
	old, ok := m.core[output]
	if ok && (value > old-0.001 && value < old+0.001) {
		return
	}
	m.core[output] = value
	m.setting.SetString(gsKeyBrightness, m.string())
}

func (m *brightnessMapManager) get(output string) (float64, error) {
	m.locker.Lock()
	defer m.locker.Unlock()
	v, ok := m.core[output]
	if !ok {
		return 0, fmt.Errorf("Unknown output: %s", output)
	}
	return v, nil
}

func (m *brightnessMapManager) reset() error {
	m.locker.Lock()
	defer m.locker.Unlock()
	data := m.setting.GetString(gsKeyBrightness)
	if len(data) == 0 {
		m.core = make(map[string]float64)
		return nil
	}
	return json.Unmarshal([]byte(data), &m.core)
}

func (m *brightnessMapManager) string() string {
	v, _ := jsonMarshal(m.core)
	return v
}

func jsonMarshal(v interface{}) (string, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func validBrightnessValue(v float64) bool {
	if v < 0 || v > 1 {
		return false
	}
	return true
}
