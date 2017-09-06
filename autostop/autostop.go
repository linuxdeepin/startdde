package autostop

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"pkg.deepin.io/lib/log"
)

type Manager struct {
	logger *log.Logger
}

func LaunchAutostopScripts(logger *log.Logger) error {
	if logger == nil {
		return fmt.Errorf("Logger is nil")
	}

	var m = Manager{
		logger: logger,
	}

	m.launchScripts(m.getAutostopScripts())
	return nil
}

func (m *Manager) launchScripts(scripts []string) {
	for _, script := range scripts {
		m.logger.Info("[Autostop] will launch:", script)
		out, err := exec.Command(script).CombinedOutput()
		if err != nil {
			m.logger.Warningf("[Autostop] failed to launch %s: %v, %v",
				script, string(out), err)
		}
	}
}

func (m *Manager) getAutostopScripts() []string {
	var dirs = []string{
		path.Join(os.Getenv("HOME"), ".config", "autostop"),
		"/etc/xdg/autostop",
	}
	var scripts []string
	for _, dir := range dirs {
		tmp, err := doScanScripts(dir)
		if err != nil {
			m.logger.Warning("[Autostop] failed to scan dir:", dir, err)
			continue
		}
		scripts = append(scripts, tmp...)
	}
	return scripts
}

func doScanScripts(dir string) ([]string, error) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scripts []string
	for _, finfo := range finfos {
		if finfo.IsDir() ||
			(finfo.Mode().Perm()&os.FileMode(0111) == 0) {
			continue
		}
		scripts = append(scripts, path.Join(dir, finfo.Name()))
	}
	return scripts, nil
}
