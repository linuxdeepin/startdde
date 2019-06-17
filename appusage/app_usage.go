package appusage

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"time"

	ue "github.com/linuxdeepin/go-dbus-factory/com.deepin.userexperience.daemon"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("startdde/appusage")

const (
	dbusServiceName = "com.deepin.AppUsage"
	dbusPath        = "/com/deepin/AppUsage"
	dbusInterface   = dbusServiceName
)

func Start() error {
	appUsage, err := newAppUsage()
	if err != nil {
		return err
	}
	service, err := dbusutil.NewSessionService()
	if err != nil {
		return err
	}
	err = service.Export(dbusPath, appUsage)
	if err != nil {
		return err
	}
	return service.RequestName(dbusServiceName)
}

type appUsage struct {
	mu         sync.Mutex
	nextId     uint64
	enabled    bool
	cfgModTime time.Time
	m          map[uint64]*appInfo
	ueDaemon   *ue.Daemon

	methods *struct {
		NotifyAppOpened func() `in:"name" out:"id"`
		NotifyAppClosed func() `in:"id"`
	}
}

type appInfo struct {
	Name  string
	Start time.Time
}

func (au *appUsage) GetInterfaceName() string {
	return dbusInterface
}

func newAppUsage() (*appUsage, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	au := &appUsage{
		nextId: 1,
	}
	au.m = make(map[uint64]*appInfo)
	au.ueDaemon = ue.NewDaemon(sysBus)
	return au, nil
}

func (au *appUsage) NotifyAppOpened(name string) (uint64, *dbus.Error) {
	id, err := au.notifyAppOpened(name)
	return id, dbusutil.ToError(err)
}

func (au *appUsage) notifyAppOpened(name string) (uint64, error) {
	if name == "" {
		return 0, errors.New("invalid name")
	}

	au.mu.Lock()
	defer au.mu.Unlock()

	enabled, err := au.isEnabled()
	if err != nil {
		return 0, err
	}
	logger.Debug("enabled:", enabled)

	if !enabled {
		return 0, nil
	}

	id := au.nextId
	au.nextId++
	au.m[id] = &appInfo{
		Name:  name,
		Start: time.Now(),
	}
	return id, nil
}

func (au *appUsage) NotifyAppClosed(id uint64) *dbus.Error {
	err := au.notifyAppClosed(id)
	return dbusutil.ToError(err)
}

func (au *appUsage) notifyAppClosed(id uint64) error {
	if id == 0 {
		return nil
	}

	au.mu.Lock()
	defer au.mu.Unlock()

	enabled, err := au.isEnabled()
	if err != nil {
		return err
	}
	logger.Debug("enabled:", enabled)

	appInfo, ok := au.m[id]
	if !ok {
		return errors.New("invalid id")
	}
	delete(au.m, id)

	if !enabled {
		return nil
	}

	msg := &message{
		Type: "app-closed",
		Body: messageBody{
			Name:  appInfo.Name,
			Start: appInfo.Start,
		},
	}
	msgJson, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return au.ueDaemon.PostMessage(0, string(msgJson))
}

type message struct {
	Type string // app-closed
	Body messageBody
}

type messageBody struct {
	Name  string
	Start time.Time
}

type config struct {
	Enabled map[uint]bool // key is uid
}

func loadConfig(filename string, cfg *config) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, cfg)
	return err
}

const configFile = "/var/lib/deepin-user-experience/config.json"

func (ue *appUsage) isEnabled() (bool, error) {
	// get mod time
	var modTime time.Time
	fileInfo, err := os.Stat(configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
	} else {
		modTime = fileInfo.ModTime()
	}

	var cfg config
	if modTime != ue.cfgModTime {
		// config file changed
		if fileInfo != nil {
			err = loadConfig(configFile, &cfg)
			if err != nil {
				logger.Warning("failed to load config:", err)
			}
		}
	} else {
		// config file not changed
		return ue.enabled, nil
	}

	enabled := cfg.Enabled[uint(os.Getuid())]
	// save cache
	ue.cfgModTime = modTime
	ue.enabled = enabled

	return enabled, nil
}
