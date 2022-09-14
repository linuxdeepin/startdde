// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UnitTestSuite struct {
	suite.Suite
	m *Manager
}

func (s *UnitTestSuite) SetupSuite() {
	var err error
	s.m = &Manager{}
	s.m.service, err = dbusutil.NewSessionService()
	if err != nil {
		s.T().Skip(fmt.Sprintf("failed to get service: %v", err))
	}

	s.m.sysBus, err = dbus.SystemBus()
	if err != nil {
		s.T().Skip(fmt.Sprintf("failed to get service: %v", err))
	}
}

func (s *UnitTestSuite) Test_initScreenRotation() {
	s.m.initScreenRotation()
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

func Test_getLspci(t *testing.T) {
	out0, err0 := exec.Command("lspci").Output()
	args := []string{}
	out1, err1 := getLspci(args)
	assert.Equal(t, string(out0), out1)
	assert.Equal(t, err0, err1)
}

func Test_detectDrmSupportGamma(t *testing.T) {
	service, err := dbusutil.NewSessionService()
	if err != nil {
		return
	}
	xConn, err := x.NewConn()
	if err != nil {
		return
	}
	_xConn = xConn
	m := newManager(service)
	m.unsupportGammaDrmList = []string{}
	out0, err0 := exec.Command("lspci").Output()
	sup, err1 := m.detectDrmSupportGamma()
	assert.Equal(t, err0, err1)
	if strings.Contains(string(out0), "VGA") {
		assert.True(t, sup)
	} else {
		assert.False(t, sup)
	}
}
