## [Unreleased]


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
