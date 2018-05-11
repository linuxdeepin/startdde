/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package wm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"pkg.deepin.io/lib/strv"
	"pkg.deepin.io/lib/xdg/basedir"
)

// CardInfo the display/graphics card id
type CardInfo struct {
	VendorID string
	DevID    string
}

// CardInfos the card id list
type CardInfos []*CardInfo

const (
	swCardPath = "deepin/deepin-wm-switcher/cards.ini"
)

// String format card infos to string
func (infos CardInfos) String() string {
	data, _ := json.Marshal(infos)
	return string(data)
}

func (infos CardInfos) genCardConfig() string {
	size := len(infos)
	if size == 0 {
		return "[cards]\nsize=0\n"
	}

	var lines []string
	lines = append(lines, "[cards]")
	for i, info := range infos {
		lines = append(lines, fmt.Sprintf("%d\\dev_id=%s", i+1, info.DevID))
		lines = append(lines, fmt.Sprintf("%d\\vendor_id=%s", i+1, info.VendorID))
	}
	contents := strings.Join(lines, "\n")
	contents += fmt.Sprintf("\nsize=%d\n", size)
	return contents
}

func getCardInfosPath() string {
	return filepath.Join(basedir.GetUserConfigDir(), swCardPath)
}

func getCardInfos() (CardInfos, error) {
	outs, err := exec.Command("lspci", "-nn").CombinedOutput()
	if err != nil {
		if len(outs) != 0 {
			err = fmt.Errorf("%s", string(outs))
		}
		return nil, err
	}

	var infos CardInfos
	cardReg := regexp.MustCompile(" (vga|3d).*(display|graphics|controller)")
	idReg := regexp.MustCompile("\\[(\\w{4}):(\\w{4})\\]")
	lines := strings.Split(string(outs), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		tmp := strings.ToLower(line)
		idxs := cardReg.FindStringIndex(tmp)
		if idxs == nil {
			continue
		}
		idxs = idReg.FindStringIndex(tmp)
		if len(idxs) != 2 {
			continue
		}

		info := CardInfo{
			VendorID: string(tmp[idxs[0]+1 : idxs[0]+1+4]),
			DevID:    string(tmp[idxs[0]+6 : idxs[0]+6+4]),
		}
		infos = append(infos, &info)
	}
	return infos, nil
}

func loadCardInfosFromFile(filename string) (CardInfos, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(contents), "\n")

	tmp := strv.Strv(lines).FilterEmpty()
	cardSize, err := getCardConfigSize(tmp)
	if err != nil {
		return nil, err
	}

	if len(tmp) < 2+cardSize*2 {
		return nil, fmt.Errorf("Invalid card config format")
	}

	if !strings.Contains(tmp[0], "cards") {
		return nil, fmt.Errorf("Invalid card config format")
	}

	var infos CardInfos
	idx := 1
	for i := 1; i < len(tmp); {
		if idx > cardSize {
			break
		}
		vv1 := strings.Split(tmp[i], fmt.Sprintf("%d\\dev_id=", idx))
		vv2 := strings.Split(tmp[i+1], fmt.Sprintf("%d\\vendor_id=", idx))
		if len(vv1) != 2 || len(vv2) != 2 {
			i += 2
			idx++
			fmt.Println("Invalid format for card id:", tmp[i], tmp[i+1])
			continue
		}
		info := CardInfo{
			VendorID: vv2[1],
			DevID:    vv1[1],
		}
		infos = append(infos, &info)
		idx++
		i += 2
	}
	return infos, nil
}

func getCardConfigSize(lines strv.Strv) (int, error) {
	length := len(lines)
	for i := length - 1; i > 0; i-- {
		if lines[i] == "" {
			continue
		}

		if !strings.Contains(lines[i], "size=") {
			return 0, fmt.Errorf("Invalid card config format")
		}
		v := strings.TrimSpace(lines[i])
		list := strings.Split(v, "size=")
		if len(list) != 2 {
			return 0, fmt.Errorf("Invalid card config format")
		}

		s, err := strconv.ParseInt(list[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return int(s), nil
	}
	return 0, fmt.Errorf("Invalid card config format")
}

func doSaveCardInfos(filename, data string) error {
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(data), 0644)
}
