package swapsched

import (
	"fmt"
	"os"
	"sync"
	"time"

	"pkg.deepin.io/lib/log"
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

type TuneConfig struct {
	RootCGroup string // 默认的root cgroup path , 需要外部程序提前配置好为uid可以操控的,且需要有cpu,memory,freezer3个control
	MemoryLock bool   // 是否调用MLockAll
}

type Dispatcher struct {
	sync.Mutex

	cfg TuneConfig
	cnt int

	activeXID int

	activeApp    *UIApp
	inactiveApps []*UIApp
}

func NewDispatcher(cfg TuneConfig) (*Dispatcher, error) {
	d := &Dispatcher{
		cfg:       cfg,
		cnt:       0,
		activeXID: -1,
	}

	if err := d.checkCGroups(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Dispatcher) checkCGroups() error {
	groups := []string{
		joinCGPath(memoryCtrl, d.cfg.RootCGroup),
		joinCGPath(freezerCtrl, d.cfg.RootCGroup),
	}
	for _, path := range groups {
		_, err := os.Stat(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) counter() int {
	d.Lock()
	d.cnt = d.cnt + 1
	d.Unlock()
	return d.cnt
}

func (d *Dispatcher) NewApp(desktop string) (*UIApp, error) {
	cgroup := fmt.Sprintf("%s/%d", d.cfg.RootCGroup, d.counter())
	app, err := newApp(cgroup, desktop)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (d *Dispatcher) AddApp(app *UIApp) {
	logger.Debug("Dispatcher.AddApp", app)
	d.Lock()
	d.inactiveApps = append(d.inactiveApps, app)
	d.Unlock()
}

func (d *Dispatcher) ActiveWindowHandler(pid int, xid int) {
	// pid != 0
	d.Lock()
	defer d.Unlock()

	if d.activeXID == xid {
		return
	}
	d.activeXID = xid

	if d.activeApp != nil && d.activeApp.HasChild(pid) {
		return
	}

	var newActive *UIApp
	for _, app := range d.inactiveApps {
		if app.HasChild(pid) {
			newActive = app
			break
		}
	}
	d.setActiveApp(newActive)
	d.balance()
}

func (d *Dispatcher) setActiveApp(activeApp *UIApp) {
	if d.activeApp == activeApp {
		return
	}

	var inactiveAppsTemp []*UIApp
	if d.activeApp != nil {
		inactiveAppsTemp = append(inactiveAppsTemp, d.activeApp)
	}
	for _, app := range d.inactiveApps {
		if app == activeApp {
			continue
		}
		inactiveAppsTemp = append(inactiveAppsTemp, app)
	}

	d.inactiveApps = inactiveAppsTemp
	d.activeApp = activeApp
}

func (d *Dispatcher) sample() MemInfo {
	var info MemInfo
	info.TotalRSSFree, info.TotalUsedSwap = getSystemMemoryInfo()
	info.n = len(d.inactiveApps)

	for _, app := range d.inactiveApps {
		rss, swap := app.MemoryInfo()
		info.InactiveAppsRSS += rss
		info.InactiveAppsSwap += swap
	}

	if d.activeApp != nil {
		rss, swap := d.activeApp.MemoryInfo()
		info.ActiveAppRSS = rss
		info.ActiveAppSwap = swap
	}
	return info
}

var debugBalance bool

func init() {
	if os.Getenv("DEBUG_SWAP_SCHED_BALANCE") == "1" {
		debugBalance = true
	}
}

func (d *Dispatcher) balance() {
	info := d.sample()

	if debugBalance {
		if d.activeApp == nil {
			logger.Debugf("no active app (active win: %d)\n%s\n", d.activeXID, info)
		} else {
			logger.Debugf("active app %q(%q) (%dMB,%dMB)\n%s\n",
				d.activeApp.desktop,
				d.activeApp.cgroup,
				info.ActiveAppRSS/MB, info.ActiveAppSwap/MB,
				info)
		}
	}

	freezeUIApps(d.cfg.RootCGroup)
	defer thawUIApps(d.cfg.RootCGroup)

	err := setLimitRSS(d.cfg.RootCGroup, info.UIAppsLimit())
	if err != nil {
		logger.Warning("SetUIAppsLimit failed:", err)
	}

	if d.activeApp != nil {
		err = d.activeApp.SetLimitRSS(info.ActiveAppLimit())
		if err != nil {
			logger.Warning("SetActtiveAppLimit failed:", d.activeApp, err)
		}
	}

	inactiveAppLimit := info.InactiveAppLimit()

	var liveApps []*UIApp
	for _, app := range d.inactiveApps {
		if !app.IsLive() {
			logger.Debugf("Dispatcher.balance remove %s from inactiveApps", app)
			continue
		}
		err = app.SetLimitRSS(inactiveAppLimit)
		if err != nil {
			fmt.Println("SetActtiveAppLimit failed:", app, err)
		}
		liveApps = append(liveApps, app)
	}
	d.inactiveApps = liveApps
}

func (d *Dispatcher) Balance() {
	for {
		time.Sleep(time.Second)
		d.Lock()
		d.balance()
		d.Unlock()
	}
}

type MemInfo struct {
	TotalRSSFree     uint64 //当前一共可用的物理内存
	TotalUsedSwap    uint64 //当前已使用的Swap内存
	ActiveAppRSS     uint64 //活跃App占用的物理内存
	ActiveAppSwap    uint64 //活跃App占用的Swap内存
	InactiveAppsRSS  uint64 //除活跃App外所有APP一共占用的物理内存 (不含DDE等非UI APP组件)
	InactiveAppsSwap uint64 //除活跃App外所有APP一共占用的Swap内存 (不含DDE等非UI APP组件)

	n int
}

func (info MemInfo) UIAppsLimit() uint64 {
	return info.TotalRSSFree + info.ActiveAppRSS + info.InactiveAppsRSS
}

func (info MemInfo) String() string {
	str := fmt.Sprintf("TotalFree %dMB, SwapUsed: %dMB\n",
		info.TotalRSSFree/MB, info.TotalUsedSwap/MB)
	str += fmt.Sprintf("UI Limit: %dMB\nActive App Limit: %dMB (need %dMB)\nInAcitve App Limit %dMB (%d need %dMB)",
		info.UIAppsLimit()/MB,
		info.ActiveAppLimit()/MB,
		(info.ActiveAppRSS+info.ActiveAppSwap)/MB,
		info.InactiveAppLimit()/MB,
		info.n,
		(info.InactiveAppsRSS+info.InactiveAppsSwap)/MB,
	)
	return str
}

const MB = 1000 * 1000

func (info MemInfo) ActiveAppLimit() uint64 {
	if info.ActiveAppRSS == 0 {
		return 0
	}

	max := func(a, b uint64) uint64 {
		if a > b {
			return a
		}
		return b
	}

	// 逐步满足ActiveApp的内存需求，但上限由UIAppsLimit()决定(cgroup本身会保证,不需要在这里做截断)。
	return max(info.ActiveAppRSS+100*MB, info.UIAppsLimit()-100*MB)
}

func (info MemInfo) InactiveAppLimit() uint64 {
	// TODO: 是否需要除以app数量?
	//优先保证ActiveApp有机会完全加载到RSS中
	min := func(a, b uint64) uint64 {
		if a < b {
			return a
		}
		return b
	}
	activeMemory := info.ActiveAppRSS + info.ActiveAppSwap
	return min(info.UIAppsLimit()-activeMemory, info.TotalRSSFree-activeMemory)
}
