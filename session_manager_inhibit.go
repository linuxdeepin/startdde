package main

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	signalInhibitorAdded   = "InhibitorAdded"
	signalInhibitorRemoved = "InhibitorRemoved"
)

//  The flags parameter must include at least one of the following:
//
//    1: Inhibit logging out
//    2: Inhibit user switching
//    4: Inhibit suspending the session or computer
//    8: Inhibit the session being marked as idle
func (m *SessionManager) Inhibit(sender dbus.Sender, appId string, toplevelXid uint32, reason string,
	flags uint32) (inhibitCookie uint32, busErr *dbus.Error) {

	ih, err := m.inhibitManager.add(string(sender), appId, toplevelXid, reason, flags)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}

	ihPath := ih.getPath()
	err = m.service.Export(ihPath, ih)
	if err != nil {
		_, err0 := m.inhibitManager.remove(string(sender), ih.id)
		if err0 != nil {
			logger.Warningf("failed to remove inhibitor %v: %v", ih.id, err0)
		}
		return 0, dbusutil.ToError(err)
	}

	err = m.service.Emit(m, signalInhibitorAdded, ihPath)
	if err != nil {
		logger.Warning(err)
	}

	return ih.id, nil
}

func (m *SessionManager) IsInhibited(flags uint32) (bool, *dbus.Error) {
	v := m.inhibitManager.isInhibited(flags)
	return v, nil
}

func (m *SessionManager) Uninhibit(sender dbus.Sender, inhibitCookie uint32) *dbus.Error {
	ih, err := m.inhibitManager.remove(string(sender), inhibitCookie)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = m.service.StopExport(ih)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = m.service.Emit(m, signalInhibitorRemoved, ih.getPath())
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *SessionManager) GetInhibitors() ([]dbus.ObjectPath, *dbus.Error) {
	paths := m.inhibitManager.getInhibitorsPaths()
	return paths, nil
}

func (m *SessionManager) initInhibitManager() {
	m.inhibitManager.inhibitors = make(map[uint32]*Inhibitor)
}

type InhibitManager struct {
	nextId     uint32
	inhibitors map[uint32]*Inhibitor // key is inhibitor id
	mu         sync.Mutex
}

func (im *InhibitManager) getNewId() (uint32, error) {
	const countMax = 1000
	count := 0
	for count < countMax {
		_, ok := im.inhibitors[im.nextId]
		if ok {
			im.nextId++
		} else {
			id := im.nextId
			im.nextId++
			return id, nil
		}
		count++
	}
	return 0, errors.New("failed to get new id")
}

func (im *InhibitManager) add(sender, appId string, toplevelXid uint32, reason string, flags uint32) (*Inhibitor, error) {

	im.mu.Lock()
	defer im.mu.Unlock()

	id, err := im.getNewId()
	if err != nil {
		return nil, err
	}

	ih := &Inhibitor{
		createAt:    time.Now(),
		id:          id,
		sender:      sender,
		appId:       appId,
		reason:      reason,
		flags:       flags,
		toplevelXid: toplevelXid,
	}
	im.inhibitors[id] = ih
	return ih, nil
}

func (im *InhibitManager) remove(sender string, id uint32) (*Inhibitor, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	ih := im.inhibitors[id]
	if ih == nil {
		return nil, errors.New("not found inhibitor")
	}

	if ih.sender != sender {
		return nil, errors.New("sender not match")
	}

	delete(im.inhibitors, id)
	return ih, nil
}

func (im *InhibitManager) isInhibited(flags uint32) bool {
	im.mu.Lock()
	defer im.mu.Unlock()

	for _, ih := range im.inhibitors {
		if ih.flags&flags != 0 {
			return true
		}
	}
	return false
}

func (im *InhibitManager) getInhibitorsPaths() []dbus.ObjectPath {
	im.mu.Lock()
	defer im.mu.Unlock()

	ihs := make([]*Inhibitor, 0, len(im.inhibitors))
	for _, ih := range im.inhibitors {
		ihs = append(ihs, ih)
	}
	sort.Slice(ihs, func(i, j int) bool {
		// less
		return ihs[i].createAt.Before(ihs[j].createAt)
	})
	paths := make([]dbus.ObjectPath, len(ihs))
	for idx, ih := range ihs {
		paths[idx] = ih.getPath()
	}
	return paths
}

func (im *InhibitManager) handleNameLost(name string) *Inhibitor {
	im.mu.Lock()
	defer im.mu.Unlock()

	for id, ih := range im.inhibitors {
		if ih.sender == name {
			delete(im.inhibitors, id)
			return ih
		}
	}
	return nil
}

type Inhibitor struct {
	id     uint32
	sender string

	createAt    time.Time
	appId       string
	reason      string
	flags       uint32
	toplevelXid uint32
}

func (i *Inhibitor) GetInterfaceName() string {
	return "com.deepin.SessionManager.Inhibitor"
}

func (i *Inhibitor) getPath() dbus.ObjectPath {
	return dbus.ObjectPath("/com/deepin/SessionManager/Inhibitors/Inhibitor_" +
		strconv.FormatUint(uint64(i.id), 10))
}

func (i *Inhibitor) GetAppId() (string, *dbus.Error) {
	return i.appId, nil
}

func (i *Inhibitor) GetClientId() (dbus.ObjectPath, *dbus.Error) {
	// TODO
	return "/", dbusutil.ToError(errors.New("not implement client"))
}

func (i *Inhibitor) GetReason() (string, *dbus.Error) {
	return i.reason, nil
}

func (i *Inhibitor) GetFlags() (uint32, *dbus.Error) {
	return i.flags, nil
}

func (i *Inhibitor) GetToplevelXid() (uint32, *dbus.Error) {
	return i.toplevelXid, nil
}
