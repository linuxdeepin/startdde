/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package wm

import (
	"path/filepath"

	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/xdg/basedir"
)

const (
	sogouConfigPath = "sogou-qimpanel/main.conf"

	sgGroupBase   = "base"
	sgKeyCurtSkin = "CurtSogouSkinName"

	sgDefaultSkin = "默认皮肤"
)

func getSogouConfigPath() string {
	return filepath.Join(basedir.GetUserConfigDir(), sogouConfigPath)
}

func setSogouSkin(skin, filename string) error {
	kfile := keyfile.NewKeyFile()
	kfile.LoadFromFile(filename)

	v, err := kfile.GetString(sgGroupBase, sgKeyCurtSkin)
	if err != nil {
		return err
	}

	if skin == v {
		return nil
	}

	kfile.SetString(sgGroupBase, sgKeyCurtSkin, skin)
	return kfile.SaveToFile(filename)
}

func getSogouSkin(filename string) (string, error) {
	kfile := keyfile.NewKeyFile()
	err := kfile.LoadFromFile(filename)
	if err != nil {
		return "", err
	}

	return kfile.GetString(sgGroupBase, sgKeyCurtSkin)
}
