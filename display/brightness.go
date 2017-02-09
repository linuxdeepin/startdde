package display

import (
	"fmt"
	"pkg.deepin.io/dde/startdde/display/brightness"
)

func (dpy *Manager) SaveBrightness() {
	dpy.setting.SetString(gsKeyBrightness, jsonMarshal(dpy.Brightness))
}

func (dpy *Manager) SupportedBacklight(name string) bool {
	info := dpy.outputInfos.QueryByName(name)
	if len(info.Name) == 0 {
		return false
	}
	return brightness.SupportBacklight(info.Id, dpy.conn)
}

func (dpy *Manager) initBrightness() {
	value := dpy.setting.GetString(gsKeyBrightness)
	tmp := make(map[string]float64)
	if len(value) != 0 {
		err := jsonUnmarshal(value, &tmp)
		if err != nil {
			logger.Warningf("[initBrightness] unmarshal (%s) failed: %v",
				value, err)
		}
	}

	setter := dpy.setting.GetString(gsKeySetter)
	for _, info := range dpy.outputInfos {
		if _, ok := tmp[info.Name]; ok {
			continue
		}

		b, err := brightness.Get(setter, info.Id, dpy.conn)
		if err == nil {
			tmp[info.Name] = b
		}
	}

	for k, v := range tmp {
		dpy.doSetBrightness(v, k)
	}
}

func (dpy *Manager) doSetBrightness(value float64, name string) error {
	info := dpy.outputInfos.QueryByName(name)
	if len(info.Name) == 0 {
		return fmt.Errorf("Invalid output name: %s", name)
	}

	err := brightness.Set(value, dpy.setting.GetString(gsKeySetter),
		info.Id, dpy.conn)
	if err != nil {
		logger.Error("Set brightness to %v for %s failed: %v", value, name, err)
		return err
	}

	oldValue := dpy.Brightness[name]
	if oldValue == value {
		return nil
	}

	// update brightness of the output
	dpy.Brightness[name] = value
	dpy.setPropBrightness(dpy.Brightness)
	return nil
}
