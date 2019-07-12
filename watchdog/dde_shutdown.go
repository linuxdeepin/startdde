package watchdog

import (
	"os/exec"
	"time"
)

const (
	ddeShutdownServiceName = "com.deepin.dde.shutdownFront"
	ddeShutdownCommand     = "dde-shutdown"
	ddeShutdownTaskName    = ddeShutdownCommand
)

func isDdeShutdownRunning() (bool, error) {
	return isDBusServiceExist(ddeShutdownServiceName)
}

func newDdeShutdownTask() *taskInfo {
	t := newTaskInfo(ddeShutdownTaskName, isDdeShutdownRunning, launchDdeShutdown)
	t.launchDelay = 500 * time.Millisecond
	return t
}

func launchDdeShutdown() error {
	return launchCommand(ddeShutdownCommand, []string{"-d"}, ddeShutdownTaskName)
}

func launchCommand(command string, args []string, name string) error {
	var cmd = exec.Command(command, args...)
	err := cmd.Start()
	if err != nil {
		logger.Warningf("failed to start %s: %v", name, err)
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warningf("%s exit with error: %v", name, err)
		}
	}()
	return nil
}
