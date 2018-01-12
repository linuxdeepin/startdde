package swapsched

import (
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"pkg.deepin.io/lib/cgroup"
)

const (
	SystemCGroupRoot = "/sys/fs/cgroup"
)

const KB = 1024
const MB = 1024 * KB
const GB = 1024 * MB

func joinCGPath(args ...string) string {
	return path.Join(SystemCGroupRoot, path.Join(args...))
}

func getRSSUsed(memCtl *cgroup.Controller) uint64 {
	var cache, rss, mappedFile uint64
	memCtl.GetStats([]string{"cache", "rss", "mapped_file"},
		&cache, &rss, &mappedFile)
	return cache + rss + mappedFile
}

func setSoftLimit(memCtl *cgroup.Controller, v uint64) error {
	return memCtl.SetValueUint64(softLimitInBytes, v)
}

func cancelSoftLimit(memCtl *cgroup.Controller) error {
	return memCtl.SetValueInt64(softLimitInBytes, -1)
}

func setHardLimit(memCtl *cgroup.Controller, v uint64) error {
	return memCtl.SetValueUint64(limitInBytes, v)
}

type ProcMemoryInfo struct {
	MemTotal     uint64
	MemAvailable uint64
	SwapTotal    uint64
	SwapFree     uint64
}

func getProcMemoryInfo() (memInfo ProcMemoryInfo) {
	MemTotal, MemAvailable, SwapTotal, SwapFree := "MemTotal", "MemAvailable", "SwapTotal", "SwapFree"
	vs := ParseMemoryStatKB("/proc/meminfo",
		MemTotal, MemAvailable, SwapTotal, SwapFree)

	memInfo.MemTotal = vs[MemTotal]
	memInfo.MemAvailable = vs[MemAvailable]
	memInfo.SwapTotal = vs[SwapTotal]
	memInfo.SwapFree = vs[SwapFree]
	return
}

func getProcessesSwap(pids ...int) uint64 {
	VmSwap := "VmSwap"
	ret := uint64(0)
	for _, pid := range pids {
		ret += ParseMemoryStatKB(fmt.Sprintf("/proc/%d/status", pid), VmSwap)[VmSwap]
	}
	return ret
}

func toLines(v []byte, hasErr error) []string {
	if hasErr != nil {
		return nil
	}
	var ret []string
	for _, line := range strings.Split(string(v), "\n") {
		if line != "" {
			ret = append(ret, line)
		}
	}
	return ret
}

// ParseMemoryStatKB parse fields with KB suffix in /proc/self/status, /proc/meminfo
func ParseMemoryStatKB(filePath string, keys ...string) map[string]uint64 {
	ret := make(map[string]uint64)
	for _, line := range toLines(ioutil.ReadFile(filePath)) {
		fields := strings.Split(line, ":")
		if len(fields) != 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		for _, ikey := range keys {
			if key == ikey {
				value := strings.TrimSpace(fields[1])
				value = strings.Replace(value, " kB", "", -1)
				v, _ := strconv.ParseUint(value, 10, 64)
				ret[key] = v * KB
				if len(ret) >= len(keys) {
					return ret
				}
			}
		}
	}
	return ret
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
