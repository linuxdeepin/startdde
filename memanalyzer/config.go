package memanalyzer

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"pkg.deepin.io/lib/xdg/basedir"
	"sync"
)

var (
	_memDB     map[string]uint64
	_memLocker sync.Mutex
)

func init() {
	_memLocker.Lock()
	db, err := loadConfig(getConfigPath())
	if err != nil {
		_memDB = make(map[string]uint64)
	} else {
		_memDB = db
	}
	_memLocker.Unlock()
}

// DumpDB dump config contents
func DumpDB() string {
	_memLocker.Lock()
	defer _memLocker.Unlock()
	if len(_memDB) == 0 {
		return ""
	}

	var ret = "{"
	for k, v := range _memDB {
		ret += fmt.Sprintf("\"%s\": %v,", k, v)
	}
	data := []byte(ret)
	data[len(data)-1] = '}'
	return string(data)
}

func setDB(k string, v uint64) {
	_memLocker.Lock()
	defer _memLocker.Unlock()
	tmp := _memDB[k]
	if tmp == v {
		return
	}
	_memDB[k] = v
}

func getDB(k string) uint64 {
	_memLocker.Lock()
	defer _memLocker.Unlock()
	return _memDB[k]
}

func doSaveDB(filename string) error {
	_memLocker.Lock()
	defer _memLocker.Unlock()
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}
	var w bytes.Buffer
	err = gob.NewEncoder(&w).Encode(_memDB)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, w.Bytes(), 0644)
}

func loadConfig(filename string) (map[string]uint64, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var db = make(map[string]uint64)
	r := bytes.NewReader(contents)
	err = gob.NewDecoder(r).Decode(&db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getConfigPath() string {
	return filepath.Join(basedir.GetUserCacheDir(),
		"deepin", "startdde", "memanalyzer.db")
}
