/*
 * Copyright (C) 2020 ~ 2022 Uniontech Software Technology Co.,Ltd
 *
 * Author:     dengbo <dengbo@uniontech.com>
 *
 * Maintainer: dengbo <dengbo@uniontech.com>
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
	out1, err1 := getLspci()
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
