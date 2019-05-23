package video_card

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"pkg.deepin.io/lib/xdg/basedir"
)

// CardInfo the display/graphics card id
type CardInfo struct {
	VendorID string
	DevID    string
}

// CardInfos the card id list
type CardInfos []*CardInfo

var cardInfosPath = filepath.Join(basedir.GetUserConfigDir(), "deepin/startdde/cards.json")

func getCardInfos() (CardInfos, error) {
	outs, err := exec.Command("lspci", "-nn").CombinedOutput()
	if err != nil {
		if len(outs) != 0 {
			err = fmt.Errorf("%s", outs)
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
	var cardInfos CardInfos
	err = json.Unmarshal(contents, &cardInfos)
	if err != nil {
		return nil, err
	}
	return cardInfos, nil
}

func doSaveCardInfos(filename string, cardInfos CardInfos) error {
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}
	data, err := json.Marshal(cardInfos)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0644)
}

func IsCardChange() (change bool) {
	actualCardInfos, err := getCardInfos()
	if err != nil {
		logger.Warning("failed to get card info:", err)
		return true
	}

	cacheCardInfos, err := loadCardInfosFromFile(cardInfosPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("failed to load card info from config file:", err)
		}
		change = true
	} else {
		// load cacheCardInfos ok
		if !reflect.DeepEqual(actualCardInfos, cacheCardInfos) {
			// card change
			change = true
		}
	}

	if change {
		err = doSaveCardInfos(cardInfosPath, actualCardInfos)
		if err != nil {
			logger.Warning("failed to save card infos:", err)
		}
	}
	return change
}
