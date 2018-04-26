package memanalyzer

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"pkg.deepin.io/lib/strv"
)

// GetProcessMemory get process used memory from config
func GetProcessMemory(name string) (uint64, error) {
	v := getDB(name)
	if v == 0 {
		return 0, fmt.Errorf("not found the process: %s", name)
	}
	return v, nil
}

// GetCGroupMemory get these process in cgroup used memory
func GetCGroupMemory(cgroupName string) (uint64, error) {
	list, err := getPidsInCGroup(cgroupName)
	if err != nil {
		return 0, err
	}
	return sumPidsMemory(list), nil
}

// GetPidMemory get the process used memory
func GetPidMemory(pid uint16) (uint64, error) {
	list, err := getProcessList(pid)
	if err != nil {
		fmt.Println("Failed to get process list from cgroup:", err)
		return sumMemByPid(pid)
	}
	return sumPidsMemory(list), nil
}

//SaveProcessMemory save process memory used info
func SaveProcessMemory(name string, mem uint64) error {
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

		v, err := getInteger(line)
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

func getInteger(line string) (uint64, error) {
	list := strings.Split(line, " ")
	list = strv.Strv(list).FilterEmpty()
	if len(list) != 3 {
		return 0, fmt.Errorf("bad format: %s", line)
	}
	return strconv.ParseUint(list[1], 10, 64)
}
