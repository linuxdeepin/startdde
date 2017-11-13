package swapsched

import (
	"strconv"
	"strings"
)

type UIApp struct {
	cgroup  string
	limit   uint64
	state   AppState
	desktop string
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
	pids := getCGroupPIDs(memoryCtrl, app.cgroup)
	if len(pids) == 0 {
		app.maybeDestroy()
		return false
	}
	for _, pid0 := range pids {
		if pid0 == pid {
			return true
		}
	}
	return false
}

// MemoryInfo 返回 RSS 以及 Swap使用量 (目前数据不对)
func (app *UIApp) MemoryInfo() (uint64, uint64) {
	if !app.IsLive() {
		return 0, 0
	}

	used := toUint64(readCGroupFile(memoryCtrl, app.cgroup, "memory.usage_in_bytes"))
	for _, line := range toLines(readCGroupFile(memoryCtrl, app.cgroup, "memory.stat")) {
		const rss = "rss "
		const totalActiveAnon = "total_active_anon "
		if strings.HasPrefix(line, rss) {
			v, _ := strconv.ParseUint(line[len(rss):], 10, 64)
			if v == 0 {
				app.maybeDestroy()
				break
			}
		} else if strings.HasPrefix(line, totalActiveAnon) {
			v, _ := strconv.ParseUint(line[len(totalActiveAnon):], 10, 64)
			if v > used {
				break
			}
			return used - v, v
		}
	}
	return used, 0
}

func (app *UIApp) SetLimitRSS(v uint64) error {
	if !app.IsLive() {
		return nil
	}
	app.limit = v
	return setLimitRSS(app.cgroup, v)
}

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

func newApp(subCGroup string, desktop string) (*UIApp, error) {
	err := cgCreate(memoryCtrl, subCGroup)
	if err != nil {
		return nil, err
	}
	return &UIApp{
		cgroup:  subCGroup,
		limit:   0,
		state:   AppStateInit,
		desktop: desktop,
	}, nil
}
