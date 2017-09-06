package autostop

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestScriptsScan(t *testing.T) {
	convey.Convey("Test scripts scanner", t, func() {
		var rets = []string{
			"testdata/scripts/hello.sh",
			"testdata/scripts/ls.sh",
		}

		scripts, _ := doScanScripts("./testdata/scripts")
		convey.So(rets, convey.ShouldResemble, scripts)
	})
}
