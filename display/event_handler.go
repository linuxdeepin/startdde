package display

import (
	"github.com/BurntSushi/xgb/xproto"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	displayEventDelay = "DDE_DISPLAY_EVENT_DELAY"
)

var (
	eventHandleLocker sync.Mutex
	eventHandleDelay  time.Duration = 500
)

type eventTaskInfo struct {
	running bool
	width   uint16
	height  uint16

	configTimestamp xproto.Timestamp

	stop       chan struct{}
	stopLocker sync.Mutex

	handler func(uint16, uint16, xproto.Timestamp)
}

func init() {
	v := os.Getenv(displayEventDelay)
	if len(v) == 0 {
		return
	}
	delay, _ := strconv.ParseUint(v, 10, 64)
	if delay == 0 {
		return
	}
	eventHandleDelay = time.Duration(delay)
}

func (dpy *Display) handleScreenEvent(width, height uint16,
	config xproto.Timestamp) {
	eventHandleLocker.Lock()
	defer eventHandleLocker.Unlock()

	dpy.setPropScreenWidth(width)
	dpy.setPropScreenHeight(height)
	GetDisplayInfo().update()

	curPlan := dpy.QueryCurrentPlanName()
	logger.Debug("[listener] Screen event:", width, height, LastConfigTimeStamp, config)
	logger.Debugf("[Listener] current display config: %#v\n", dpy.cfg)
	if LastConfigTimeStamp < config {
		LastConfigTimeStamp = config
		if dpy.cfg == nil || dpy.cfg.CurrentPlanName != curPlan {
			logger.Info("Detect New ConfigTimestmap, try reset changes, current plan:", curPlan)
			if dpy.cfg != nil && len(curPlan) == 0 {
				dpy.cfg.CurrentPlanName = curPlan
			} else {
				dpy.ResetChanges()
				dpy.SwitchMode(dpy.DisplayMode, dpy.cfg.Plans[dpy.cfg.CurrentPlanName].DefaultOutput)
			}
		}
	}

	if len(curPlan) != 0 {
		//sync Monitor's state
		for _, m := range dpy.Monitors {
			m.updateInfo()
		}
		//changePrimary will try set an valid primary if dpy.Primary invalid
		dpy.changePrimary(dpy.Primary, true)
		dpy.mapTouchScreen()
	}
}

func newEventTaskInfo(width, height uint16, config xproto.Timestamp,
	handler func(uint16, uint16, xproto.Timestamp)) *eventTaskInfo {
	if handler == nil {
		return nil
	}

	var info = eventTaskInfo{
		width:           width,
		height:          height,
		configTimestamp: config,
	}
	info.stop = make(chan struct{})
	info.handler = handler
	return &info
}

func (info *eventTaskInfo) Execute() {
	if info.running {
		return
	}
	info.running = true
	select {
	case <-info.stop:
		return
	case <-time.NewTimer(time.Millisecond * eventHandleDelay).C:
		info.running = false
		info.handler(info.width, info.height, info.configTimestamp)
	}

	info.stopLocker.Lock()
	defer info.stopLocker.Unlock()
	if info.stop != nil {
		close(info.stop)
		info.stop = nil
	}
}

func (info *eventTaskInfo) Terminate() {
	if !info.running {
		return
	}
	info.stopLocker.Lock()
	defer info.stopLocker.Unlock()
	if info.stop != nil {
		close(info.stop)
		info.stop = nil
	}
}
