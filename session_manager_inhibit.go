package main

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"pkg.deepin.io/lib/dbus"
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
func (m *SessionManager) Inhibit(dMsg dbus.DMessage, appId string, toplevelXid uint32, reason string, flags uint32) (inhibitCookie uint32, err error) {
	sender := dMsg.GetSender()
	ih, err := m.inhibitManager.add(sender, appId, toplevelXid, reason, flags)
	if err != nil {
		return 0, err
	}

	err = dbus.InstallOnSession(ih)
	if err != nil {
		_, _ = m.inhibitManager.remove(sender, ih.id)
		return 0, err
	}

	err = dbus.Emit(m, signalInhibitorAdded, ih.getPath())
	if err != nil {
		logger.Warning(err)
	}

	return ih.id, nil
}

func (m *SessionManager) IsInhibited(flags uint32) (bool, error) {
	v := m.inhibitManager.isInhibited(flags)
	return v, nil
}

func (m *SessionManager) Uninhibit(dMsg dbus.DMessage, inhibitCookie uint32) error {
	sender := dMsg.GetSender()
	ih, err := m.inhibitManager.remove(sender, inhibitCookie)
	if err != nil {
		return err
	}
	dbus.UnInstallObject(ih)
	err = dbus.Emit(m, signalInhibitorRemoved, ih.getPath())
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *SessionManager) GetInhibitors() ([]dbus.ObjectPath, error) {
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

func (i *Inhibitor) getPath() dbus.ObjectPath {
	return dbus.ObjectPath("/com/deepin/SessionManager/Inhibitors/Inhibitor_" +
		strconv.FormatUint(uint64(i.id), 10))
}

func (i *Inhibitor) GetDBusInfo() dbus.DBusInfo {
	return dbus.DBusInfo{
		Dest:       "com.deepin.SessionManager",
		ObjectPath: string(i.getPath()),
		Interface:  "com.deepin.SessionManager.Inhibitor",
	}
}

// interface com.deepin.SessionManager.Inhibitor

func (i *Inhibitor) GetAppId() (string, error) {
	return i.appId, nil
}

func (i *Inhibitor) GetClientId() (dbus.ObjectPath, error) {
	// TODO
	return "/", errors.New("not implement client")
}

func (i *Inhibitor) GetReason() (string, error) {
	return i.reason, nil
}

func (i *Inhibitor) GetFlags() (uint32, error) {
	return i.flags, nil
}

func (i *Inhibitor) GetToplevelXid() (uint32, error) {
	return i.toplevelXid, nil
}
