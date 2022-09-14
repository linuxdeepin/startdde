// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Code generated by "dbusutil-gen -output display_dbusutil.go -import github.com/godbus/dbus,github.com/linuxdeepin/go-x11-client,github.com/linuxdeepin/go-lib/strv -type Manager,Monitor manager.go monitor.go"; DO NOT EDIT.

package display

import (
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-lib/strv"
)

func (v *Manager) setPropMonitors(value []dbus.ObjectPath) (changed bool) {
	if !objPathsEqual(v.Monitors, value) {
		v.Monitors = value
		v.emitPropChangedMonitors(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedMonitors(value []dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "Monitors", value)
}

func (v *Manager) setPropCustomIdList(value []string) {
	v.CustomIdList = value
	v.emitPropChangedCustomIdList(value)
}

func (v *Manager) emitPropChangedCustomIdList(value []string) error {
	return v.service.EmitPropertyChanged(v, "CustomIdList", value)
}

func (v *Manager) setPropHasChanged(value bool) (changed bool) {
	if v.HasChanged != value {
		v.HasChanged = value
		v.emitPropChangedHasChanged(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedHasChanged(value bool) error {
	return v.service.EmitPropertyChanged(v, "HasChanged", value)
}

func (v *Manager) setPropDisplayMode(value byte) (changed bool) {
	if v.DisplayMode != value {
		v.DisplayMode = value
		v.emitPropChangedDisplayMode(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedDisplayMode(value byte) error {
	return v.service.EmitPropertyChanged(v, "DisplayMode", value)
}

func (v *Manager) setPropBrightness(value map[string]float64) {
	v.Brightness = value
	v.emitPropChangedBrightness(value)
}

func (v *Manager) emitPropChangedBrightness(value map[string]float64) error {
	return v.service.EmitPropertyChanged(v, "Brightness", value)
}

func (v *Manager) setPropTouchscreens(value dxTouchscreens) {
	v.Touchscreens = value
	v.emitPropChangedTouchscreens(value)
}

func (v *Manager) emitPropChangedTouchscreens(value dxTouchscreens) error {
	return v.service.EmitPropertyChanged(v, "Touchscreens", value)
}

func (v *Manager) setPropTouchscreensV2(value dxTouchscreens) {
	v.TouchscreensV2 = value
	v.emitPropChangedTouchscreensV2(value)
}

func (v *Manager) emitPropChangedTouchscreensV2(value dxTouchscreens) error {
	return v.service.EmitPropertyChanged(v, "TouchscreensV2", value)
}

func (v *Manager) setPropTouchMap(value map[string]string) {
	v.TouchMap = value
	v.emitPropChangedTouchMap(value)
}

func (v *Manager) emitPropChangedTouchMap(value map[string]string) error {
	return v.service.EmitPropertyChanged(v, "TouchMap", value)
}

func (v *Manager) setPropCurrentCustomId(value string) (changed bool) {
	if v.CurrentCustomId != value {
		v.CurrentCustomId = value
		v.emitPropChangedCurrentCustomId(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedCurrentCustomId(value string) error {
	return v.service.EmitPropertyChanged(v, "CurrentCustomId", value)
}

func (v *Manager) setPropPrimary(value string) (changed bool) {
	if v.Primary != value {
		v.Primary = value
		v.emitPropChangedPrimary(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedPrimary(value string) error {
	return v.service.EmitPropertyChanged(v, "Primary", value)
}

func (v *Manager) setPropPrimaryRect(value x.Rectangle) (changed bool) {
	if v.PrimaryRect != value {
		v.PrimaryRect = value
		v.emitPropChangedPrimaryRect(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedPrimaryRect(value x.Rectangle) error {
	return v.service.EmitPropertyChanged(v, "PrimaryRect", value)
}

func (v *Manager) setPropScreenWidth(value uint16) (changed bool) {
	if v.ScreenWidth != value {
		v.ScreenWidth = value
		v.emitPropChangedScreenWidth(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedScreenWidth(value uint16) error {
	return v.service.EmitPropertyChanged(v, "ScreenWidth", value)
}

func (v *Manager) setPropScreenHeight(value uint16) (changed bool) {
	if v.ScreenHeight != value {
		v.ScreenHeight = value
		v.emitPropChangedScreenHeight(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedScreenHeight(value uint16) error {
	return v.service.EmitPropertyChanged(v, "ScreenHeight", value)
}

func (v *Manager) setPropMaxBacklightBrightness(value uint32) (changed bool) {
	if v.MaxBacklightBrightness != value {
		v.MaxBacklightBrightness = value
		v.emitPropChangedMaxBacklightBrightness(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedMaxBacklightBrightness(value uint32) error {
	return v.service.EmitPropertyChanged(v, "MaxBacklightBrightness", value)
}

func (v *Manager) setPropColorTemperatureMode(value int32) (changed bool) {
	if v.ColorTemperatureMode != value {
		v.ColorTemperatureMode = value
		v.emitPropChangedColorTemperatureMode(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedColorTemperatureMode(value int32) error {
	return v.service.EmitPropertyChanged(v, "ColorTemperatureMode", value)
}

func (v *Manager) setPropColorTemperatureManual(value int32) (changed bool) {
	if v.ColorTemperatureManual != value {
		v.ColorTemperatureManual = value
		v.emitPropChangedColorTemperatureManual(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedColorTemperatureManual(value int32) error {
	return v.service.EmitPropertyChanged(v, "ColorTemperatureManual", value)
}

func (v *Monitor) setPropID(value uint32) (changed bool) {
	if v.ID != value {
		v.ID = value
		v.emitPropChangedID(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedID(value uint32) error {
	return v.service.EmitPropertyChanged(v, "ID", value)
}

func (v *Monitor) setPropName(value string) (changed bool) {
	if v.Name != value {
		v.Name = value
		v.emitPropChangedName(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedName(value string) error {
	return v.service.EmitPropertyChanged(v, "Name", value)
}

func (v *Monitor) setPropConnected(value bool) (changed bool) {
	if v.Connected != value {
		v.Connected = value
		v.emitPropChangedConnected(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedConnected(value bool) error {
	return v.service.EmitPropertyChanged(v, "Connected", value)
}

func (v *Monitor) setPropManufacturer(value string) (changed bool) {
	if v.Manufacturer != value {
		v.Manufacturer = value
		v.emitPropChangedManufacturer(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedManufacturer(value string) error {
	return v.service.EmitPropertyChanged(v, "Manufacturer", value)
}

func (v *Monitor) setPropModel(value string) (changed bool) {
	if v.Model != value {
		v.Model = value
		v.emitPropChangedModel(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedModel(value string) error {
	return v.service.EmitPropertyChanged(v, "Model", value)
}

func (v *Monitor) setPropRotations(value []uint16) (changed bool) {
	if !uint16SliceEqual(v.Rotations, value) {
		v.Rotations = value
		v.emitPropChangedRotations(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedRotations(value []uint16) error {
	return v.service.EmitPropertyChanged(v, "Rotations", value)
}

func (v *Monitor) setPropReflects(value []uint16) (changed bool) {
	if !uint16SliceEqual(v.Reflects, value) {
		v.Reflects = value
		v.emitPropChangedReflects(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedReflects(value []uint16) error {
	return v.service.EmitPropertyChanged(v, "Reflects", value)
}

func (v *Monitor) setPropBestMode(value ModeInfo) (changed bool) {
	if v.BestMode != value {
		v.BestMode = value
		v.emitPropChangedBestMode(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedBestMode(value ModeInfo) error {
	return v.service.EmitPropertyChanged(v, "BestMode", value)
}

func (v *Monitor) setPropModes(value []ModeInfo) (changed bool) {
	if !modeInfosEqual(v.Modes, value) {
		v.Modes = value
		v.emitPropChangedModes(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedModes(value []ModeInfo) error {
	return v.service.EmitPropertyChanged(v, "Modes", value)
}

func (v *Monitor) setPropPreferredModes(value []ModeInfo) (changed bool) {
	if !modeInfosEqual(v.PreferredModes, value) {
		v.PreferredModes = value
		v.emitPropChangedPreferredModes(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedPreferredModes(value []ModeInfo) error {
	return v.service.EmitPropertyChanged(v, "PreferredModes", value)
}

func (v *Monitor) setPropMmWidth(value uint32) (changed bool) {
	if v.MmWidth != value {
		v.MmWidth = value
		v.emitPropChangedMmWidth(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedMmWidth(value uint32) error {
	return v.service.EmitPropertyChanged(v, "MmWidth", value)
}

func (v *Monitor) setPropMmHeight(value uint32) (changed bool) {
	if v.MmHeight != value {
		v.MmHeight = value
		v.emitPropChangedMmHeight(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedMmHeight(value uint32) error {
	return v.service.EmitPropertyChanged(v, "MmHeight", value)
}

func (v *Monitor) setPropEnabled(value bool) (changed bool) {
	if v.Enabled != value {
		v.Enabled = value
		v.emitPropChangedEnabled(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedEnabled(value bool) error {
	return v.service.EmitPropertyChanged(v, "Enabled", value)
}

func (v *Monitor) setPropX(value int16) (changed bool) {
	if v.X != value {
		v.X = value
		v.emitPropChangedX(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedX(value int16) error {
	return v.service.EmitPropertyChanged(v, "X", value)
}

func (v *Monitor) setPropY(value int16) (changed bool) {
	if v.Y != value {
		v.Y = value
		v.emitPropChangedY(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedY(value int16) error {
	return v.service.EmitPropertyChanged(v, "Y", value)
}

func (v *Monitor) setPropWidth(value uint16) (changed bool) {
	if v.Width != value {
		v.Width = value
		v.emitPropChangedWidth(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedWidth(value uint16) error {
	return v.service.EmitPropertyChanged(v, "Width", value)
}

func (v *Monitor) setPropHeight(value uint16) (changed bool) {
	if v.Height != value {
		v.Height = value
		v.emitPropChangedHeight(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedHeight(value uint16) error {
	return v.service.EmitPropertyChanged(v, "Height", value)
}

func (v *Monitor) setPropRotation(value uint16) (changed bool) {
	if v.Rotation != value {
		v.Rotation = value
		v.emitPropChangedRotation(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedRotation(value uint16) error {
	return v.service.EmitPropertyChanged(v, "Rotation", value)
}

func (v *Monitor) setPropReflect(value uint16) (changed bool) {
	if v.Reflect != value {
		v.Reflect = value
		v.emitPropChangedReflect(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedReflect(value uint16) error {
	return v.service.EmitPropertyChanged(v, "Reflect", value)
}

func (v *Monitor) setPropRefreshRate(value float64) (changed bool) {
	if v.RefreshRate != value {
		v.RefreshRate = value
		v.emitPropChangedRefreshRate(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedRefreshRate(value float64) error {
	return v.service.EmitPropertyChanged(v, "RefreshRate", value)
}

func (v *Monitor) setPropBrightness(value float64) (changed bool) {
	if v.Brightness != value {
		v.Brightness = value
		v.emitPropChangedBrightness(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedBrightness(value float64) error {
	return v.service.EmitPropertyChanged(v, "Brightness", value)
}

func (v *Monitor) setPropCurrentRotateMode(value uint8) (changed bool) {
	if v.CurrentRotateMode != value {
		v.CurrentRotateMode = value
		v.emitPropChangedCurrentRotateMode(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedCurrentRotateMode(value uint8) error {
	return v.service.EmitPropertyChanged(v, "CurrentRotateMode", value)
}

func (v *Monitor) setPropCurrentMode(value ModeInfo) (changed bool) {
	if v.CurrentMode != value {
		v.CurrentMode = value
		v.emitPropChangedCurrentMode(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedCurrentMode(value ModeInfo) error {
	return v.service.EmitPropertyChanged(v, "CurrentMode", value)
}

func (v *Monitor) setPropCurrentFillMode(value string) (changed bool) {
	if v.CurrentFillMode != value {
		v.CurrentFillMode = value
		v.emitPropChangedCurrentFillMode(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedCurrentFillMode(value string) error {
	return v.service.EmitPropertyChanged(v, "CurrentFillMode", value)
}

func (v *Monitor) setPropAvailableFillModes(value strv.Strv) (changed bool) {
	if !v.AvailableFillModes.Equal(value) {
		v.AvailableFillModes = value
		v.emitPropChangedAvailableFillModes(value)
		return true
	}
	return false
}

func (v *Monitor) emitPropChangedAvailableFillModes(value strv.Strv) error {
	return v.service.EmitPropertyChanged(v, "AvailableFillModes", value)
}
