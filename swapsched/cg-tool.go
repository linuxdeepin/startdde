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

func setHardLimit(memCtl *cgroup.Controller, v uint64) error {
	return memCtl.SetValueUint64(limitInBytes, v)
}

// getSystemMemoryInfo 返回 系统可用内存, 系统已用Swap
func getSystemMemoryInfo() (uint64, uint64, uint64) {
	MemTotal, MemAvailable, SwapTotal, SwapFree := "MemTotal", "MemAvailable", "SwapTotal", "SwapFree"
	vs := ParseMemoryStatKB("/proc/meminfo",
		MemTotal, MemAvailable, SwapTotal, SwapFree)
	return vs[MemTotal], vs[MemAvailable], vs[SwapTotal] - vs[SwapFree]
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
