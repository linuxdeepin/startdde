package swapsched

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/log"
)

// 模块主要是利用cgroup提供的功能，使应用程序(ui-app)之间进行内存竞争（不要与DE进行竞争),
//
// 所有的系统状态都在 dispatch.sample() 中进行同一获取. (根据SamplePeroid定期执行)
// 所有的系统调整都在 dispatch.balance() 中进行. (根据SamplePeroid定期执行)
// 此外X11的root window变化会导致, dispatch.ActiveWindowHandler激活间接触发一次dispatch.balance

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

const (
	softLimitInBytes = "soft_limit_in_bytes"
	limitInBytes     = "limit_in_bytes"
)

const ActiveAppBonus = 100 * MB      // 当前激活APP的限制补偿,值越大恢复越快. 但会导致Inactive压力过大
const ActiveAppSWAPRatioInLimit = 10 // 计算ActiveAppLimit的时候会加上(其使用的Swap/此ratio)
const MinimumLimit = 5 * MB          // 内存限制的最小值, 尽量与正常UIAPP的最小值匹配.
const MaximumLimitPlus = 500 * MB    // plus TotalRSSFree, 避免某个UIApp用尽UserSpace的内存,导致僵死无法切换ActiveApp, 从而使swap-sched失效.
const DefaultSamplePeriod = 1        // 默认的数据调整周期
const KernelCacheReserve = 400 * MB  //至少预留多少内存给kernel
const DESoftLimit = 800 * MB         // DE 组的内存用量软限制

const enableSwapTotalMin = 1 * GB // 使 dispatcher 启用的最小 swap 总空间

type Config struct {
	UIAppsCGroup string // sessionID@dde/uiapps
	DECGroup     string // sessionID@dde/DE
	SamplePeroid int    // unit in second // 影响balance采样周期. 值越大系统负载更多

	DisableMemAvailMin uint64 // 使 dispatcher 禁用的最小可用内存，当 dispatcher 被启用时， 如果可用内存大于这个值，dispatcher 被禁用。
	EnableMemAvailMax  uint64 // 使 dispatcher 启用的最大可用内存，当 dispatcher 被禁用时， 如果可用内存小于这个值，dispatcher 被启用。
}

type Dispatcher struct {
	sync.Mutex
	cfg       Config
	cnt       uint32
	activeXID int
	enabled   bool

	uiAppsCg     *cgroup.Cgroup
	activeApp    *UIApp
	inactiveApps []*UIApp

	deCg *cgroup.Cgroup
}

func NewDispatcher(cfg Config) (*Dispatcher, error) {
	if cfg.SamplePeroid <= 0 {
		cfg.SamplePeroid = DefaultSamplePeriod
	}
	deCg := cgroup.NewCgroup(cfg.DECGroup)
	deCg.AddController(cgroup.Memory)

	uiAppsCg := cgroup.NewCgroup(cfg.UIAppsCGroup)
	uiAppsCg.AddController(cgroup.Freezer)
	uiAppsCg.AddController(cgroup.Memory)
	uiAppsCg.AddController(cgroup.Blkio)

	d := &Dispatcher{
		cfg:       cfg,
		uiAppsCg:  uiAppsCg,
		deCg:      deCg,
		cnt:       0,
		activeXID: -1,
		enabled:   false,
	}

	if !d.testCgroups() {
		return nil, errors.New("controllers of cgroup not all exist")
	}
	return d, nil
}

func (d *Dispatcher) testCgroups() bool {
	return d.deCg.AllExist() && d.uiAppsCg.AllExist()
}

func (d *Dispatcher) GetDECGroup() string {
	return d.cfg.DECGroup
}

func (d *Dispatcher) counter() uint32 {
	d.Lock()
	d.cnt++
	result := d.cnt
	d.Unlock()
	return result
}

type AppResourcesLimit struct {
	MemHardLimit  uint64
	BlkioReadBPS  uint64
	BlkioWriteBPS uint64
}

func (d *Dispatcher) NewApp(desc string, limit *AppResourcesLimit) (*UIApp, error) {
	seqNum := d.counter()
	appCg := d.uiAppsCg.NewChildGroup(strconv.FormatUint(uint64(seqNum), 10))
	app, err := newApp(seqNum, appCg, desc, limit)
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

func (d *Dispatcher) shouldApplyLimit(memInfo ProcMemoryInfo) bool {
	if memInfo.SwapTotal < enableSwapTotalMin {
		return false
	}

	if d.enabled {
		if memInfo.MemAvailable > d.cfg.DisableMemAvailMin {
			// cancel limit
			return false
		}
		return true

	} else {
		if memInfo.MemAvailable < d.cfg.EnableMemAvailMax {
			return true
		}
		// cancel limit
		return false
	}
}

// sample() 在SamplePeroid的周期下被执行, 所有状态更新的函数都只应该在这里被触发.
func (d *Dispatcher) sample() (MemInfo, bool) {
	var info MemInfo
	procMemInfo := getProcMemoryInfo()
	shouldApplyLimit := d.shouldApplyLimit(procMemInfo)
	info.TotalRAM = procMemInfo.MemTotal
	info.TotalRSSFree = procMemInfo.MemAvailable
	info.TotalUsedSwap = procMemInfo.SwapTotal - procMemInfo.SwapFree

	info.n = len(d.inactiveApps)

	if shouldApplyLimit {
		for _, app := range d.inactiveApps {
			app.Update()
			info.InactiveAppsRSS += app.rssUsed
		}

		if d.activeApp != nil {
			d.activeApp.Update()
			info.ActiveAppRSS = d.activeApp.rssUsed
			if info.TotalUsedSwap != 0 {
				info.ActiveAppSWAP = getProcessesSwap(d.activeApp.pids...)
			} else {
				info.ActiveAppSWAP = 0
			}
		}

	} else {
		// only update pids of apps
		for _, app := range d.inactiveApps {
			app.updatePids()
		}
		if d.activeApp != nil {
			d.activeApp.updatePids()
		}
	}
	return info, shouldApplyLimit
}

var debugBalance bool

func init() {
	if os.Getenv("DEBUG_SWAP_SCHED_BALANCE") == "1" {
		debugBalance = true
	}
}

func (d *Dispatcher) cancelLimit() {
	logger.Debug("cancel limit")

	var err error
	appsMemCtl := d.uiAppsCg.GetController(cgroup.Memory)
	err = cancelSoftLimit(appsMemCtl)
	if err != nil {
		logger.Warning("failed to cancel soft limit for uiapps cgroup:", err)
	}

	if d.activeApp != nil {
		err = d.activeApp.cancelLimitRSS()
		if err != nil {
			logger.Warningf("failed to cancel soft limit for %s: %v", d.activeApp, err)
		}
	}

	for _, app := range d.inactiveApps {
		err = app.cancelLimitRSS()
		if err != nil {
			logger.Warningf("failed to cancel soft limit for %s: %v", app, err)
		}
	}

	deMemCtl := d.deCg.GetController(cgroup.Memory)
	err = cancelSoftLimit(deMemCtl)
	if err != nil {
		logger.Warning("failed to cancel soft limit for DE cgroup:", err)
	}
}

func (d *Dispatcher) balance() {
	info, shouldApplyLimit := d.sample()

	if shouldApplyLimit != d.enabled {
		// value changed
		d.enabled = shouldApplyLimit

		if !shouldApplyLimit {
			d.cancelLimit()
		}
	}

	// remove dead app
	var liveApps []*UIApp
	for _, app := range d.inactiveApps {
		if !app.IsLive() {
			logger.Debugf("Dispatcher.balance remove %s from inactiveApps", app)
			continue
		}
		liveApps = append(liveApps, app)
	}
	d.inactiveApps = liveApps

	if !shouldApplyLimit {
		return
	}

	if debugBalance {
		if d.activeApp == nil {
			logger.Debugf("no active app (active win: %d)\n%s\n", d.activeXID, info)
		} else {
			logger.Debugf("active app %q(%d) %dMB\n%s\n",
				d.activeApp.desc,
				d.activeApp.seqNum,
				info.ActiveAppRSS/MB,
				info)
		}
	}

	// apply limit
	appsFreezerCtl := d.uiAppsCg.GetController(cgroup.Freezer)
	err := appsFreezerCtl.SetValueString("state", "FROZEN")
	if err != nil {
		logger.Warning(err)
	} else {
		defer func() {
			err := appsFreezerCtl.SetValueString("state", "THAWED")
			if err != nil {
				logger.Warning(err)
			}
		}()
	}

	appsMemCtl := d.uiAppsCg.GetController(cgroup.Memory)
	err = setSoftLimit(appsMemCtl, info.TailorLimit(info.UIAppsTotalLimit()))
	if err != nil {
		logger.Warning("failed to set soft limit for uiapps cgroup:", err)
	}

	if d.activeApp != nil {
		err = d.activeApp.SetLimitRSS(info.TailorLimit(info.ActiveAppLimit()))
		if err != nil {
			logger.Warningf("failed to set soft limit for active %s: %v ", d.activeApp, err)
		}
	}

	for _, app := range d.inactiveApps {
		err = app.SetLimitRSS(info.TailorLimit(info.InactiveAppLimit(app.rssUsed)))
		if err != nil {
			logger.Warningf("failed to set soft limit for inactive %s: %v", app, err)
		}
	}

	deMemCtl := d.deCg.GetController(cgroup.Memory)
	err = setSoftLimit(deMemCtl, DESoftLimit)
	if err != nil {
		logger.Warning("failed to set soft limit for DE cgroup:", err)
	}
}

func (d *Dispatcher) Balance() {
	delay := time.Second * time.Duration(d.cfg.SamplePeroid)
	for {
		time.Sleep(delay)
		d.Lock()
		d.balance()
		d.Unlock()
	}
}

func (d *Dispatcher) GetAppsSeqDescMap() map[uint32]string {
	d.Lock()

	length := len(d.inactiveApps)
	if d.activeApp != nil {
		length++
	}

	ret := make(map[uint32]string, length)

	if d.activeApp != nil {
		ret[d.activeApp.seqNum] = d.activeApp.desc
	}
	for _, app := range d.inactiveApps {
		ret[app.seqNum] = app.desc
	}

	d.Unlock()
	return ret
}

type MemInfo struct {
	TotalRAM      uint64 //　物理内存总大小
	TotalRSSFree  uint64 //当前一共可用的物理内存
	TotalUsedSwap uint64

	ActiveAppRSS    uint64 //ActiveApp占用的物理内存
	ActiveAppSWAP   uint64 //ActiveApp的Swap使用量
	InactiveAppsRSS uint64 //InactiveApps一共占用的物理内存.

	n int
}

func (info MemInfo) TailorLimit(v uint64) uint64 {
	//对最大值做出限制，避免严重影响DE
	free := info.TotalRAM -
		KernelCacheReserve -
		uint64(info.n)*MinimumLimit -
		MinimumLimit*2
	return min(free, v)
}

// InactiveAppLimit 根据当前可用RSS以及ActiveApp所需RSS计算最小的限制值.
func (info MemInfo) ActiveAppLimit() uint64 {
	swap := info.ActiveAppSWAP / uint64(ActiveAppSWAPRatioInLimit)
	// ActiveApp有大量swap，但RSS较小的情况, 可能会出现inactiveApp反转优先级了．
	// 因此这里加上一定的swap使用量．

	return max(info.ActiveAppRSS+ActiveAppBonus+swap, ActiveAppBonus)
}

// InactiveLimit 根据InactiveApp期望的RSS以及当前可分配的RSS按比例给予.
func (info MemInfo) InactiveAppLimit(desiredRSS uint64) uint64 {
	free := info.TotalRSSFree - info.ActiveAppLimit() - KernelCacheReserve
	if free <= 0 {
		return MinimumLimit
	}
	load := info.InactiveAppsRSS
	if load == 0 {
		return desiredRSS
	}
	return min(max(free*desiredRSS/load, MinimumLimit), desiredRSS)
}

// cgroup uiapps的总限制
func (info MemInfo) UIAppsTotalLimit() uint64 {
	v := info.TotalRSSFree + info.ActiveAppRSS + info.InactiveAppsRSS - KernelCacheReserve
	return max(MinimumLimit, v)
}

func (info MemInfo) String() string {
	str := fmt.Sprintf("TotalFree %dMB, SwapUsed: %dMB\n",
		info.TotalRSSFree/MB, info.TotalUsedSwap/MB)
	str += fmt.Sprintf("UI Limit: %dMB\nActive App Limit: %dMB (need %dMB)\n %d InAcitve Apps need %dMB",
		info.UIAppsTotalLimit()/MB,
		info.ActiveAppLimit()/MB,
		(info.ActiveAppRSS)/MB,
		info.n,
		(info.InactiveAppsRSS)/MB,
	)
	return str
}
