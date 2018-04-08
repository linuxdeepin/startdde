package swapsched

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"pkg.deepin.io/lib/cgroup"
	"pkg.deepin.io/lib/xdg/basedir"
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

func getHomeDirBlockDevice() (string, error) {
	homeDir := basedir.GetUserHomeDir()
	fileInfo, err := os.Stat(homeDir)
	if err != nil {
		return "", err
	}
	sysStat := fileInfo.Sys().(*syscall.Stat_t)
	majorNum := major(uint64(sysStat.Dev))
	minorNum := minor(uint64(sysStat.Dev))
	blockPath := fmt.Sprintf("/sys/dev/block/%d:%d", majorNum, minorNum)
	blockRealPath, err := filepath.EvalSymlinks(blockPath)
	if err != nil {
		return "", err
	}
	parentDevPath := filepath.Join(filepath.Dir(blockRealPath), "dev")
	devNum, err := ioutil.ReadFile(parentDevPath)
	devNum = bytes.TrimSpace(devNum)
	if err != nil {
		return "", err
	}
	return string(devNum), nil
}

// major and minor is copy from golang.org/x/sys/unix
// major returns the major component of a Linux device number.
func major(dev uint64) uint32 {
	major := uint32((dev & 0x00000000000fff00) >> 8)
	major |= uint32((dev & 0xfffff00000000000) >> 32)
	return major
}

// minor returns the minor component of a Linux device number.
func minor(dev uint64) uint32 {
	minor := uint32((dev & 0x00000000000000ff) >> 0)
	minor |= uint32((dev & 0x00000ffffff00000) >> 12)
	return minor
}

func setReadBPS(blkioCtl *cgroup.Controller, device string, v uint64) error {
	value := fmt.Sprintf("%s %d", device, v)
	return blkioCtl.SetValueString("throttle.read_bps_device", value)
}

func setWriteBPS(blkioCtl *cgroup.Controller, device string, v uint64) error {
	value := fmt.Sprintf("%s %d", device, v)
	return blkioCtl.SetValueString("throttle.write_bps_device", value)
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
