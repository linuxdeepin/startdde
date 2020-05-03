package watchdog

import (
	"time"
)

const (
	deepinidDaemonServiceName = "com.deepin.deepinid"
	deepinidDaemonCommand     = "/usr/lib/deepin-deepinid-daemon/deepin-deepinid-daemon"
	deepinidDaemonTaskName    = "deepinid-daemon"
)

func isDeepinidDaemonRunning() (bool, error) {
	return isDBusServiceExist(deepinidDaemonServiceName)
}

func newDeepinidDaemonTask() *taskInfo {
	t := newTaskInfo(deepinidDaemonTaskName, isDeepinidDaemonRunning, launchDeepinidDaemon)
	t.launchDelay = 500 * time.Millisecond
	return t
}

func launchDeepinidDaemon() error {
	return launchCommand(deepinidDaemonCommand, []string{}, deepinidDaemonTaskName)
}
