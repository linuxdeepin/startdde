package watchdog

import (
	"os/exec"
	"time"
)

const (
	kWinServiceName = "org.kde.KWin"
	ddeKWinCommand  = "kwin_no_scale"
)

func isDdeKWinRunning() (bool, error) {
	return isDBusServiceExist(kWinServiceName)
}

func launchDdeKWin() error {
	var cmd = exec.Command(ddeKWinCommand)
	err := cmd.Start()
	if err != nil {
		logger.Warning("failed to start dde-kwin:", err)
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning("dde-kwin exit with error:", err)
		}
	}()
	return nil
}

func newDdeKWinTask() *taskInfo {
	t := newTaskInfo(wmTaskName, isDdeKWinRunning, launchDdeKWin)
	t.launchDelay = 3 * time.Second
	return t
}
