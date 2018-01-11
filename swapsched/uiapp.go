package swapsched

import (
	"pkg.deepin.io/lib/cgroup"
)

type UIApp struct {
	seqNum  uint32
	cg      *cgroup.Cgroup
	limit   uint64
	desktop string

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
	app.pids, _ = app.cg.GetProcs(cgroup.Memory)

	if len(app.pids) == 0 {
		app.maybeDestroy()
		return
	}

	ctl := app.cg.GetController(cgroup.Memory)
	app.rssUsed = getRSSUsed(ctl)
}

func (app *UIApp) SetLimitRSS(v uint64) error {
	if !app.IsLive() {
		return nil
	}
	app.limit = v
	ctl := app.cg.GetController(cgroup.Memory)
	return setSoftLimit(ctl, v)
}

// 设置的CGroup Soft Limit值
func (app *UIApp) LimitRSS() uint64 {
	return app.limit
}
func (app *UIApp) IsLive() bool {
	return app.state != AppStateDead
}

func (app *UIApp) SetStateEnd() {
	if app.state == AppStateInit {
		logger.Debug("UIApp.SetStateEnd", app)
		app.state = AppStateEnd
	}
}

func (app *UIApp) maybeDestroy() {
	if app.state != AppStateEnd {
		return
	}
	// state End -> Dead

	app.state = AppStateDead
	logger.Debug("dead", app)

	err := app.cg.Delete(cgroup.DeleteFlagEmptyOnly)
	if err != nil {
		logger.Warningf("failed to delete cgroup for %s: %v", app, err)
	}
}

func newApp(seqNum uint32, cg *cgroup.Cgroup, desktop string, hardLimit uint64) (*UIApp, error) {
	err := cg.Create(false)
	if err != nil {
		return nil, err
	}

	if hardLimit > 0 {
		memCtl := cg.GetController(cgroup.Memory)
		err = setHardLimit(memCtl, hardLimit)
		if err != nil {
			return nil, err
		}
	}
	return &UIApp{
		seqNum:  seqNum,
		cg:      cg,
		limit:   0,
		state:   AppStateInit,
		desktop: desktop,
	}, nil
}
