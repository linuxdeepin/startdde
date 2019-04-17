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
	"pkg.deepin.io/gir/gio-2.0"
)

const (
	gsKeyTypeBool int = iota + 1
	gsKeyTypeInt
	gsKeyTypeString
)

type typeGSKeyInfo struct {
	gsKey  string
	xsKey  string
	gsType int
}

type typeGSKeyInfos []typeGSKeyInfo

var gsInfos = typeGSKeyInfos{
	{
		gsKey:  "theme-name",
		xsKey:  "Net/ThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "icon-theme-name",
		xsKey:  "Net/IconThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "fallback-icon-theme",
		xsKey:  "Net/FallbackIconTheme",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "sound-theme-name",
		xsKey:  "Net/SoundThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-theme-name",
		xsKey:  "Gtk/ThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-cursor-theme-name",
		xsKey:  "Gtk/CursorThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-font-name",
		xsKey:  "Gtk/FontName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-key-theme-name",
		xsKey:  "Gtk/KeyThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-color-palette",
		xsKey:  "Gtk/ColorPalette",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-toolbar-style",
		xsKey:  "Gtk/ToolbarStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-toolbar-icon-size",
		xsKey:  "Gtk/ToolbarIconSize",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-color-scheme",
		xsKey:  "Gtk/ColorScheme",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-preedit-style",
		xsKey:  "Gtk/IMPreeditStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-status-style",
		xsKey:  "Gtk/IMStatusStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-module",
		xsKey:  "Gtk/IMModule",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-modules",
		xsKey:  "Gtk/Modules",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-menubar-accel",
		xsKey:  "Gtk/MenuBarAccel",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "xft-hintstyle",
		xsKey:  "Xft/HintStyle",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "xft-rgba",
		xsKey:  "Xft/RGBA",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "cursor-blink-time",
		xsKey:  "Net/CursorBlinkTime",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "gtk-cursor-blink-timeout",
		xsKey:  "Net/CursorBlinkTimeout",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "double-click-time",
		xsKey:  "Net/DoubleClickTime",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "double-click-distance",
		xsKey:  "Net/DoubleClickDistance",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "dnd-drag-threshold",
		xsKey:  "Net/DndDragThreshold",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  gsKeyGtkCursorThemeSize,
		xsKey:  "Gtk/CursorThemeSize",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "gtk-timeout-initial",
		xsKey:  "Gtk/TimeoutInitial",
		gsType: gsKeyTypeInt,
	}, //deprecated
	{
		gsKey:  "gtk-timeout-repeat",
		xsKey:  "Gtk/TimeoutRepeat",
		gsType: gsKeyTypeInt,
	}, //deprecated
	{
		gsKey:  "gtk-recent-files-max-age",
		xsKey:  "Gtk/RecentFilesMaxAge",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "xft-dpi",
		xsKey:  "Xft/DPI",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "cursor-blink",
		xsKey:  "Net/CursorBlink",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "enable-event-sounds",
		xsKey:  "Net/EnableEventSounds",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "enable-input-feedback-sounds",
		xsKey:  "Net/EnableInputFeedbackSounds",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-can-change-accels",
		xsKey:  "Gtk/CanChangeAccels",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-menu-images",
		xsKey:  "Gtk/MenuImages",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-button-images",
		xsKey:  "Gtk/ButtonImages",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-enable-animations",
		xsKey:  "Gtk/EnableAnimations",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-show-input-method-menu",
		xsKey:  "Gtk/ShowInputMethodMenu",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-show-unicode-menu",
		xsKey:  "Gtk/ShowUnicodeMenu",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-auto-mnemonics",
		xsKey:  "Gtk/AutoMnemonics",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-recent-files-enabled",
		xsKey:  "Gtk/RecentFilesEnabled",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-shell-shows-app-menu",
		xsKey:  "Gtk/ShellShowsAppMenu",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "xft-antialias",
		xsKey:  "Xft/Antialias",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "xft-hinting",
		xsKey:  "Xft/Hinting",
		gsType: gsKeyTypeBool,
	},
}

func (infos typeGSKeyInfos) getInfoByGSKey(key string) *typeGSKeyInfo {
	for _, info := range infos {
		if key == info.gsKey {
			return &info
		}
	}

	return nil
}

func (infos typeGSKeyInfos) getInfoByXSKey(key string) *typeGSKeyInfo {
	for _, info := range infos {
		if key == info.xsKey {
			return &info
		}
	}

	return nil
}

func (info *typeGSKeyInfo) getKeySType() int8 {
	switch info.gsType {
	case gsKeyTypeBool, gsKeyTypeInt:
		return settingTypeInteger
	case gsKeyTypeString:
		return settingTypeString
	}

	return settingTypeInteger
}

func (info *typeGSKeyInfo) getKeyValue(s *gio.Settings) interface{} {
	switch info.gsType {
	case gsKeyTypeBool:
		v := s.GetBoolean(info.gsKey)
		if v {
			return int32(1)
		}
		return int32(0)
	case gsKeyTypeInt:
		return int32(s.GetInt(info.gsKey))
	case gsKeyTypeString:
		return s.GetString(info.gsKey)
	}

	return nil
}

func (info *typeGSKeyInfo) setKeyValue(s *gio.Settings, v interface{}) {
	switch info.gsType {
	case gsKeyTypeBool:
		tmp := v.(int32)
		if tmp == 1 {
			s.SetBoolean(info.gsKey, true)
		} else {
			s.SetBoolean(info.gsKey, false)
		}
	case gsKeyTypeInt:
		s.SetInt(info.gsKey, v.(int32))
	case gsKeyTypeString:
		s.SetString(info.gsKey, v.(string))
	}
}
