// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package wm

import (
	"path/filepath"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
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
	_ = kfile.LoadFromFile(filename)
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
