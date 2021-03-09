package display

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pkg.deepin.io/lib/xdg/basedir"
)

type ConnectInfo struct {
	Connects           map[string]bool
	LastConnectedTimes map[string]time.Time
}

var (
	cacheLocker sync.Mutex
	cacheFile   = filepath.Join(basedir.GetUserCacheDir(),
		"deepin/startdded/connectifno.cache")
)

func doReadCache(file string) (*ConnectInfo, error) {
	cacheLocker.Lock()
	defer cacheLocker.Unlock()
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	var info ConnectInfo
	decoder := gob.NewDecoder(fp)
	err = decoder.Decode(&info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func doSaveCache(info *ConnectInfo, file string) error {
	cacheLocker.Lock()
	defer cacheLocker.Unlock()
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return err
	}

	fp, err := os.Create(file)
	if err != nil {
		return err
	}
	defer fp.Close()

	encoder := gob.NewEncoder(fp)
	err = encoder.Encode(info)
	if err != nil {
		return err
	}
	return fp.Sync()
}
