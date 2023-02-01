// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os/user"
)

const (
	sessionManagerServiceName = "org.deepin.dde.SessionManager1"
	sessionManagerPath        = "/org/deepin/dde/SessionManager1"
	sessionManagerIfc         = sessionManagerServiceName
)

func (m *SessionManager) GetInterfaceName() string {
	return sessionManagerIfc
}

func (op *SessionManager) setPropName(name string) {
	switch name {
	case "CurrentUid":
		info, err := user.Current()
		if err != nil {
			logger.Infof("Get Current User Info Failed: %v", err)
			return
		}
		op.CurrentUid = info.Uid
	}
}

func (m *SessionManager) setPropStage(v int32) {
	if m.Stage != v {
		m.Stage = v
		err := m.service.EmitPropertyChanged(m, "Stage", v)
		if err != nil {
			logger.Warning(err)
		}
	}
}
