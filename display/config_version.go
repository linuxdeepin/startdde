package display

import (
	"io/ioutil"
	"os"
	"path"
)

const _confVersion = "3.3"

var (
	confVersionFile = os.Getenv("HOME") + "/.config/deepin/startdde/config.version"
)

func (dpy *Manager) checkConfigVersion() {
	if isVersionRight(_confVersion, confVersionFile) {
		return
	}

	logger.Debug("Config version not same, will delete config && write version file")
	dpy.config = &configManager{
		BaseGroup: make(map[string]*configMonitor),
		filename:  configFile,
	}

	err := os.Remove(configFile)
	if err != nil {
		logger.Warning("Failed to delete config:", err)
	}

	err = os.MkdirAll(path.Dir(confVersionFile), 0755)
	if err != nil {
		logger.Warning("Failed to mkdir:", err)
		return
	}
	err = ioutil.WriteFile(confVersionFile, []byte(_confVersion), 0644)
	if err != nil {
		logger.Warning("Failed to wirte version file:", err)
	}
	dpy.setPropDisplayMode(DisplayModeExtend)
}

func isVersionRight(version, file string) bool {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return false
	}

	return string(data) == version
}
