package memanalyzer

import (
	"bufio"
	"fmt"
	"os"
	"pkg.deepin.io/lib/strv"
	"strconv"
	"strings"
)

// GetProccessMemory get proccess used memory from config
func GetProccessMemory(name string) (uint64, error) {
	v := getDB(name)
	if v == 0 {
		return 0, fmt.Errorf("Not found the proccess: %s", name)
	}
	return v, nil
}

// GetCGroupMemory get these proccess in cgroup used memory
func GetCGroupMemory(gid string) (uint64, error) {
	list, err := getPidsInCGroup(gid)
	if err != nil {
		return 0, err
	}
	return sumPidsMemory(list), nil
}

// GetPidMemory got the proccess used memory
func GetPidMemory(pid uint16) (uint64, error) {
	list, err := getProccessList(pid)
	if err != nil {
		fmt.Println("Failed to get proccess list from cgroup:", err)
		return sumMemByPid(pid)
	}
	return sumPidsMemory(list), nil
}

//SaveProccessMemory save proccess memory used info
func SaveProccessMemory(name string, mem uint64) error {
	setDB(name, mem)
	return doSaveDB(getConfigPath())
}

func sumPidsMemory(pids []uint16) uint64 {
	var memSize uint64
	for _, v := range pids {
		s, err := sumMemByPid(v)
		if err != nil {
			fmt.Println("Failed to sum pid memory:", err)
			continue
		}
		memSize += s
	}
	return memSize
}

func sumMemByPid(pid uint16) (uint64, error) {
	return sumMemByFile(fmt.Sprintf("/proc/%v/status", pid))
}

func sumMemByFile(filename string) (uint64, error) {
	fr, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer fr.Close()

	var count = 0
	var memSize uint64
	var scanner = bufio.NewScanner(fr)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if !strings.Contains(line, "RssAnon:") &&
			!strings.Contains(line, "VmPTE:") &&
			!strings.Contains(line, "VmPMD:") {
			continue
		}

		v, err := getInterge(line)
		if err != nil {
			return 0, err
		}
		memSize += v

		count++
		if count == 3 {
			break
		}
	}

	return memSize, nil
}

func getInterge(line string) (uint64, error) {
	list := strings.Split(line, " ")
	list = strv.Strv(list).FilterEmpty()
	if len(list) != 3 {
		return 0, fmt.Errorf("Bad format: %s", line)
	}
	return strconv.ParseUint(list[1], 10, 64)
}
