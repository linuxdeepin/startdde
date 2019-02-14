package watchdog

const (
	ddeLockTaskName    = "dde-lock"
	ddeLockServiceName = "com.deepin.dde.lockFront"
)

func launchDDELock() error {
	return startService(ddeLockServiceName)
}

func newDDELock(getLockedFn func() bool) *taskInfo {
	isDDELockRunning := func() (bool, error) {
		if getLockedFn() {
			return isDBusServiceExist(ddeLockServiceName)
		} else {
			return false, errNoNeedLaunch
		}
	}
	return newTaskInfo(ddeLockTaskName, isDDELockRunning, launchDDELock)
}
