package display

import (
	"io/ioutil"
	"path"
	"pkg.linuxdeepin.com/lib/utils"
	"regexp"
	"sort"
	"strings"
)

const (
	drmCardDir = "/sys/class/drm"
)

var cardIdMap = make(map[string]string)

func sortMonitors(monitors []*Monitor) []*Monitor {
	if len(monitors) <= 1 {
		return monitors
	}

	genConnectedIdMap(drmCardDir)
	if len(cardIdMap) == 0 {
		return sortMonitorsByName(monitors)
	}
	return sortMonitorsById(monitors)
}

func sortMonitorsByName(monitors []*Monitor) []*Monitor {
	var names []string
	for _, m := range monitors {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	logger.Debug("Monitor names after sorted:", names)

	var ret []*Monitor
	for _, name := range names {
		ret = append(ret, getMonitorByName(monitors, name))
	}
	return ret
}

func sortMonitorsById(monitors []*Monitor) []*Monitor {
	var ids []string
	for _, id := range cardIdMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	logger.Debug("Monitor ids after sorted:", ids, cardIdMap)

	var ret []*Monitor
	for _, id := range ids {
		if m := getMonitorById(monitors, id); m != nil {
			ret = append(ret, m)
		}
	}
	if len(ret) == len(monitors) {
		return ret
	}

	// failed
	return sortMonitorsByName(monitors)
}

func getMonitorByName(monitors []*Monitor, name string) *Monitor {
	for _, m := range monitors {
		if m.Name == name {
			return m
		}
	}
	return nil
}

func getMonitorById(monitors []*Monitor, id string) *Monitor {
	for _, m := range monitors {
		v := getCardId(m.Name)
		if v == id {
			return m
		}
	}
	return nil
}

func getCardId(name string) string {
	array := strings.Split(name, "-")
	name = strings.Join(array, "")
	logger.Debug("[getCardId] ========= output name:", name)
	return cardIdMap[name]
}

var numReg = regexp.MustCompile(`-?[0-9]`)

func genConnectedIdMap(dir string) {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	for _, finfo := range finfos {
		name := finfo.Name()
		file := path.Join(dir, name, "edid")
		if !utils.IsFileExist(file) {
			continue
		}

		contents, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}

		if string(contents) == "" {
			continue
		}

		var idPrefix = name
		array := strings.Split(name, "-")
		if len(array) > 1 {
			name = strings.Join(array[1:], "")
			idPrefix = strings.Join(array[1:], "-")
		}

		logger.Debug("[genConnectedIdMap] -------card name:", name)
		_, ok := cardIdMap[name]
		if ok {
			continue
		}
		md5, _ := utils.SumStrMd5(string(contents))
		md5 = numReg.ReplaceAllString(idPrefix, "") + md5
		logger.Debug("[genConnectedIdMap] -------card id:", md5)
		cardIdMap[name] = md5
	}
}
