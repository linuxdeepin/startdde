package memanalyzer

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"pkg.deepin.io/lib/dbus"
)

const (
	login1Dest       = "org.freedesktop.login1"
	login1SelfPath   = "/org/freedesktop/login1/session/self"
	login1SessionIFC = login1Dest + ".Session"
)

var (
	_sessionID = ""
)

func getProcessList(pid uint16) ([]uint16, error) {
	dir, err := getCGroupDDEPath()
	if err != nil {
		return nil, err
	}

	fileInfoList, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range fileInfoList {
		if !fileInfo.IsDir() {
			continue
		}

		found, ret := isPidFound(pid, fmt.Sprintf("%s/%s/cgroup.procs",
			dir, fileInfo.Name()))
		if found {
			return ret, nil
		}
	}
	return nil, fmt.Errorf("no group found for %v", pid)
}

func getPidsInCGroup(cgroupName string) ([]uint16, error) {
	cgroupProcsFile := filepath.Join("/sys/fs/cgroup/memory", cgroupName, "cgroup.procs")
	contents, err := ioutil.ReadFile(cgroupProcsFile)
	if err != nil {
		return nil, err
	}

	var ret []string
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		ret = append(ret, line)
	}
	return strvToUint16(ret), nil
}

func isPidFound(pid uint16, filename string) (bool, []uint16) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return false, nil
	}

	var found = false
	var ret []string
	var tmp = fmt.Sprintf("%v", pid)
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		if line == tmp {
			found = true
		}
		ret = append(ret, line)
	}

	if !found {
		return false, nil
	}

	return true, strvToUint16(ret)
}

func getCGroupDDEPath() (string, error) {
	id, err := getSessionID()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/sys/fs/cgroup/memory/%s@dde/uiapps/", id), nil
}

func getSessionID() (string, error) {
	if _sessionID != "" {
		return _sessionID, nil
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	obj := conn.Object(login1Dest, login1SelfPath)
	var r dbus.Variant
	err = obj.Call("org.freedesktop.DBus.Properties.Get", 0, login1SessionIFC, "Id").Store(&r)
	if err != nil {
		return "", err
	}

	_sessionID = r.Value().(string)
	return _sessionID, nil
}

func strvToUint16(list []string) []uint16 {
	var ret []uint16
	for _, s := range list {
		v, _ := strconv.ParseUint(s, 10, 64)
		ret = append(ret, uint16(v))
	}
	return ret
}
