## [3.6.0] - 2018-12-10
*   fix: improve double clicks on touchscreen for Qt-based applications

## [3.5.0] - 2018-12-07
*   feat(fix-xauthority-perm): create .Xauthority if not exist

## [3.4.0] - 2018-11-23
*   feat: modify the meaning of allow switch
*   fix(xsettings): gtk window cursor size wrong

## [3.3.0] - 2018-11-09
*   fix(display): the disconnected output still on
*   feat(display): add touchscreen rotation supported

## [3.2.0] - 2018-10-26
*   fix(display): brightness setter auto is not good
*   fix(display): no listen xrandr event
*   chore: update changelog
*   fix: typo in `auto_launch/chinese.json`
*   chore: update changelog
*   chore: do not call gtk init
*   chore: `auto_launch/chinese.json` remove dde-control-center
*   chore: update changelog
*   fix: add booster-dtkwidget in auto launch chinese json
*   refactor: fix multiple typos
*   feat: run dde-welcome if deepin version changed
*   feat(wm): check wait for launcher wm

## [3.1.35.3] - 2018-09-07
*   fix: typo in `auto_launch/chinese.json`

## [3.1.35.2] - 2018-09-06
*   chore: do not call gtk init
*   chore: `auto_launch/chinese.json` remove dde-control-center
*   feat: run dde-welcome if deepin version changed

## [3.1.35.1] - 2018-08-31
*   fix: add booster-dtkwidget in auto launch chinese json
*   refactor: fix multiple typos

## [3.1.35] - 2018-07-31
*   fix(display): auto set scale factor to 2 in virtualbox
*   feat: allow session daemon run after dde-session-daemon part2 started

## [3.1.34] - 2018-07-23
*   chore(debian): update build depends
*   fix: test failed in the pbuilder environment
*   feat: auto set scale factor
*   chore(display): rename setGammaSize to setOutputCrtcGamma
*   chore(display): err message include stderr
*   feat: support dde-session-daemon 2 step start
*   feat: launch dde-welcome by exec
*   feat: setup environment in startdde
*   fix(display): set brightness error typo
*   chore: no import lib xgb/proto
*   chore: use go-x11-client
*   chore: do not auto launch dde-file-manager on x86 arch
*   perf(swapsched): use less cpu when swapsched is not really enabled

## [3.1.33] - 2018-06-13
*   feat(display): set default brightness of output to 1

## [3.1.32] - 2018-06-07
*   fix(wm): select wm in dde-wm-chooser but it doesn't work
*   feat: add bin fix-xauthority-perm
*   chore: update build-depends

## [3.1.31] - 2018-05-30
*   feat(wm): show osd after receive wm StartupReady signal
*   fix: can't log into session because of .Xauthority
*   feat(swapsched): check cgexec existence
*   chore: update makefile
*   chore: update makefile for arch `sw_64`

## [3.1.30] - 2018-05-23
*   fix(keyring): `check_login` infinite loop
*   fix(swapsched): data race problem

## [3.1.29] - 2018-05-14
*   fix(wm): fix genCardConfig when not found any video card
*   fix(watchdog): dde-polkit-agent running state check wrong
*   adjust `auto_launch.json`
*   fix: launch group
*   feat: wait wm launch
*   refactor: fix a typo
*   fix(debian): miss depends on libpam-gnome-keyring
*   refactor: refactor memchecker and memanalyzer
*   feat(swapsched): remove hard limit on DE group
*   feat: auto launch dde-control-center under mips64el

## [3.1.28] - 2018-04-17
*   fix(wm): CurrentWM() return `unknown` if config file not found
*   feat(startManager): add method LaunchAppWithOptions

## [3.1.27] - 2018-04-09
*   feat: add features switch for iowait and memchecker
*   fix(swapsched): cannot use sysStat.Dev (type uint32) as type uint64
*   fix: launchWait insufficient log information
*   feat(swapsched): use config of memchecker to configure swap sched dispatcher
*   chore: update memchecher min avail mem default value
*   refactor: improve memchecker debug
*   feat: add memanalyzer
*   fix: return nil if mem insufficient
*   fix: correct the action name
*   chore: fix config install path wrong
*   fix(memchecker): fix needed memory sum wrong
*   fix(memchecker): improve mem sufficient detect rule
*   refactor: refactor memchecker
*   fix(memchecker): fix warning dialog not showing again after close
*   refactor(memchecker): change the config path
*   feat: add memchecker for app launch
*   feat(watchdog): launch wm earlier after finding it dead
*   refactor: fix a typo

## [3.1.26] - 2018-03-20
*   fix: env var `SSH_AUTH_SOCK` not exported

## [3.1.25] - 2018-03-07
*   fix: optimize channel statements
*   feat(swapsched): set blkio read write limit for apps supported
*   chore: update license
*   fix: make gnome-keyring-daemon no hang
*   fix(keyring): fix crash because of dbus no replies
*   fix(watchdog): update dde polkit agent determine methods
*   fix: make keyring inited on goroutinue
*   fix(display): fix refresh rate set wrong
*   fix: improve launch failed messages
*   chore: fix gccgo compile failure
*   feat: add keyring to init login
*   chore: optimize launch config
*   feat: use new lib gsettings
*   refactor: add auto launch config
*   feat: setup environment in script deepin-session
*   add deepin-session
*   feat: initialize gnome keyring daemon and components
*   feat: add iowait to indicate cpu status

## [3.1.24] - 2018-01-25
*   fix: Adapt lintian
*   play logout sound via ALSA
*   startManager: launched hook supported
*   remove dde-readahead
*   update depends
*   refactor sound theme player call
*   add DE Component processes to DE cgroup
*   startManager: desktop key X-Deepin-MaximumRAM supported
*   improve calculating limit of InActiveApps
*   limit ActiveApp's minimum rss limit
*   consider ActiveApp's swap usage and reversing kernel cache
*   limit maximum limit for reversing more cache RAM
*   startManager: launch DE app in DE cgroup
*   add wm switcher
*   startManager: add method GetApps
*   update links in README
*   fix radeon detect failure
*   remove the depend 'deepin-wm-switcher'
*   use lib cgroup
*   simplify cgroups check
*   swapsched: turn limits on or off dynamically
*   improve description of uiapp opened with RunCommand
*   modify ldflags args, fix debug version not work
*   add wm watcher in watchdog
*   fix compile failed using gccgo
*   wm: fix wm switch not work if config incomplete
*   swapsched: do not set soft limit for DE group
*   make xsettings as a package

## [3.1.23] - 2017-12-13
*   add swap sched
*   launch app no scaling supported
*   startManager: fix method launch no files arg
*   refactor code about autostart
*   update makefile GOLDFLAGS
*   swap sched can control whether it is enabled in gsettings

## [3.1.22] - 2017-11-29
* display: fix primary rect wrong after rotation


## [3.1.21] - 2017-11-28
* display: sync primary settings from commandline
* disable logout sound if speaker muted


## [3.1.20] - 2017-11-22
* fix(display): sync primary rectangle when apply changes


## [3.1.19] - 2017-11-16
* fix primary rectangle wrong when output off
* correct deepin-wm-switcher config file path


## [3.1.18] - 2017-11-3
* reap children processes
* remove sound event cache before playing
* launch deepin-notifications on session start

## [3.1.17] - 2017-10-25
*   brightness: only call displayBl.List once in init ([4a232f17](4a232f17))
*   update soundutils event name ([634a9451](634a9451))


## [3.1.16] - 2017-10-12
### Added
* add window widget scale factor
* add virtual machine resolution corrector
* add 'autostop' to execute some shells before logout
* add option to start the app with proxychains

### Changed
* not scaled xresource dpi
* update license

### Fixed
* fix display modes index out
