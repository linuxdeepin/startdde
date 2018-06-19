package swapsched

import (
	"sync"

	"pkg.deepin.io/lib/cgroup"
)

type UIApp struct {
	seqNum uint32
	cg     *cgroup.Cgroup
	limit  uint64
	desc   string
	mu     sync.Mutex

	// 以下字段会在Update时更新.
	state   AppState
	rssUsed uint64
	pids    []int
}

type AppState int

const (
	AppStateInit AppState = iota
	AppStateEnd           // first process end, cmd.Wait() return
	AppStateDead
)

func (app *UIApp) String() string {
	return "UIApp<" + app.cg.Name() + ">"
}

func (app *UIApp) GetCGroup() string {
	return app.cg.Name()
}

func (app *UIApp) HasChild(pid int) bool {
	for _, pid0 := range app.pids {
		if pid0 == pid {
			return true
		}
	}
	return false
}

// Update 更新 rssUsed以及pids字段, 若len(pids)为0, 则尝试释放此uiapp
func (app *UIApp) Update() {
	app.updatePids()
	ctl := app.cg.GetController(cgroup.Memory)
	app.rssUsed = getRSSUsed(ctl)
}

func (app *UIApp) updatePids() {
	app.pids, _ = app.cg.GetProcs(cgroup.Memory)
	if len(app.pids) == 0 {
		app.maybeDestroy()
	}
}

func (app *UIApp) SetLimitRSS(v uint64) error {
	if !app.IsLive() {
		return nil
	}
	app.limit = v
	ctl := app.cg.GetController(cgroup.Memory)
	return setSoftLimit(ctl, v)
}

func (app *UIApp) cancelLimitRSS() error {
	ctl := app.cg.GetController(cgroup.Memory)
	return cancelSoftLimit(ctl)
}

// 设置的CGroup Soft Limit值
func (app *UIApp) LimitRSS() uint64 {
	return app.limit
}
func (app *UIApp) IsLive() bool {
	app.mu.Lock()
	result := app.state != AppStateDead
	app.mu.Unlock()
	return result
}

func (app *UIApp) SetStateEnd() {
	app.mu.Lock()

	if app.state == AppStateInit {
		logger.Debug("UIApp.SetStateEnd", app)
		app.state = AppStateEnd
	}

	app.mu.Unlock()
}

func (app *UIApp) maybeDestroy() {
	app.mu.Lock()

	if app.state != AppStateEnd {
		app.mu.Unlock()
		return
	}
	// state End -> Dead

	app.state = AppStateDead
	app.mu.Unlock()
	logger.Debug("dead", app)

	err := app.cg.Delete(cgroup.DeleteFlagEmptyOnly)
	if err != nil {
		logger.Warningf("failed to delete cgroup for %s: %v", app, err)
	}
}

func newApp(seqNum uint32, cg *cgroup.Cgroup, desc string,
	limit *AppResourcesLimit) (*UIApp, error) {

	err := cg.Create(false)
	if err != nil {
		return nil, err
	}

	if limit != nil {
		if limit.MemHardLimit > 0 {
			memCtl := cg.GetController(cgroup.Memory)
			err = setHardLimit(memCtl, limit.MemHardLimit)
			if err != nil {
				logger.Warning("failed to set hard limit:", err)
			}
		}

		if limit.BlkioReadBPS > 0 || limit.BlkioWriteBPS > 0 {
			blkioCtl := cg.GetController(cgroup.Blkio)

			var homeDevice string
			homeDevice, err = getHomeDirBlockDevice()
			if err == nil {
				if limit.BlkioReadBPS > 0 {
					err = setReadBPS(blkioCtl, homeDevice, limit.BlkioReadBPS)
					if err != nil {
						logger.Warning("failed to set read BPS:", err)
					}
				}

				if limit.BlkioWriteBPS > 0 {
					err = setWriteBPS(blkioCtl, homeDevice, limit.BlkioWriteBPS)
					if err != nil {
						logger.Warning("failed to set write BPS:", err)
					}
				}
			} else {
				logger.Warning("failed to get home dir block device:", err)
			}

		}
	}

	return &UIApp{
		seqNum: seqNum,
		cg:     cg,
		limit:  0,
		state:  AppStateInit,
		desc:   desc,
	}, nil
}
