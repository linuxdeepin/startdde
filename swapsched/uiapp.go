package swapsched

type UIApp struct {
	seqNum  uint32
	cgroup  string
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
	return "UIApp<" + app.cgroup + ">"
}

func (app *UIApp) GetCGroup() string {
	return app.cgroup
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
	app.pids = getCGroupPIDs(memoryCtrl, app.cgroup)
	if len(app.pids) == 0 {
		app.maybeDestroy()
		return
	}

	const CACHE, MAPPEDFILE, RSS = "cache ", "rss ", "mapped_file "
	vs := ParseMemoryStat(app.cgroup,
		CACHE,
		RSS,
		MAPPEDFILE,
	)
	app.rssUsed = vs[CACHE] + vs[RSS] + vs[MAPPEDFILE]
}

func (app *UIApp) SetLimitRSS(v uint64) error {
	if !app.IsLive() {
		return nil
	}
	app.limit = v
	return setLimitRSS(app.cgroup, v)
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
	cgDelete(memoryCtrl, app.cgroup)
	logger.Debug("UIApp dead", app.cgroup)
}

func newApp(seqNum uint32, subCGroup, desktop string, hardLimit uint64) (*UIApp, error) {
	err := cgCreate(memoryCtrl, subCGroup)
	if err != nil {
		return nil, err
	}

	if hardLimit > 0 {
		err = setHardLimit(subCGroup, hardLimit)
		if err != nil {
			return nil, err
		}
	}
	return &UIApp{
		seqNum:  seqNum,
		cgroup:  subCGroup,
		limit:   0,
		state:   AppStateInit,
		desktop: desktop,
	}, nil
}
