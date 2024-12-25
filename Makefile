PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = github.com/linuxdeepin/startdde
GOBUILD = go build -v $(GO_BUILD_FLAGS)
export GOPATH= $(shell go env GOPATH)

LANGUAGES = $(basename $(notdir $(wildcard misc/po/*.po)))

all: build

prepare:
	@mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX});
	@ln -snf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX};

startdde:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o startdde ${GOPKG_PREFIX}

fix-xauthority-perm:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -o fix-xauthority-perm ${GOPKG_PREFIX}/cmd/fix-xauthority-perm

out/locale/%/LC_MESSAGES/startdde.mo: misc/po/%.po
	mkdir -p $(@D)
	msgfmt -o $@ $<

translate: $(addsuffix /LC_MESSAGES/startdde.mo, $(addprefix out/locale/, ${LANGUAGES}))

pot:
	deepin-update-pot misc/po/locale_config.ini

build: prepare startdde fix-xauthority-perm translate

test: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go test -v ${GOPKG_PREFIX}

test-coverage: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go test -cover -v ${GOPKG_PREFIX} | awk '$$1 ~ "(ok|\\?)" {print $$2","$$5}' | sed "s:${CURDIR}::g" | sed 's/files\]/0\.0%/g' > coverage.csv

print_gopath: prepare
	GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}"

install:
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	install -Dm755 fix-xauthority-perm ${DESTDIR}${PREFIX}/sbin/deepin-fix-xauthority-perm
	install -d -m755 ${DESTDIR}${PREFIX}/lib/deepin-daemon/
	ln -sfv ../../bin/startdde ${DESTDIR}${PREFIX}/lib/deepin-daemon/greeter-display-daemon
	install -Dm644 misc/lightdm.conf ${DESTDIR}${PREFIX}/share/lightdm/lightdm.conf.d/60-deepin.conf
	mkdir -p ${DESTDIR}${PREFIX}/share/startdde/
	install -v -m0644 misc/filter.conf ${DESTDIR}${PREFIX}/share/startdde/
	mkdir -p $(DESTDIR)$(PREFIX)/share/glib-2.0/schemas
	install -v -m0644 misc/schemas/*.xml $(DESTDIR)$(PREFIX)/share/glib-2.0/schemas/

	mkdir -p $(DESTDIR)$(PREFIX)/share/dsg/configs/org.deepin.startdde/
	install -v -m0644 misc/dsettings/*.json $(DESTDIR)$(PREFIX)/share/dsg/configs/org.deepin.startdde/

	mkdir -pv ${DESTDIR}${PREFIX}/share/locale
	cp -r out/locale/* ${DESTDIR}${PREFIX}/share/locale

	mkdir -p $(DESTDIR)$(PREFIX)/lib/systemd/user/dde-session-initialized.target.wants/
	install -v -m0644 misc/systemd_task/dde-display-task-refresh-brightness.service $(DESTDIR)$(PREFIX)/lib/systemd/user/
	ln -s $(PREFIX)/lib/systemd/user/dde-display-task-refresh-brightness.service $(DESTDIR)$(PREFIX)/lib/systemd/user/dde-session-initialized.target.wants/dde-display-task-refresh-brightness.service


clean:
	rm -rf ${GOPATH_DIR}
	rm -f startdde
	rm -f fix-xauthority-perm

rebuild: clean build

check_code_quality: prepare
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" go vet ${GOPKG_PREFIX}

.PHONY: startdde
