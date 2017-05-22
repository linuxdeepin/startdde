PREFIX = /usr
GOPATH_DIR = gopath
GOPKG_PREFIX = pkg.deepin.io/dde/startdde

ifndef USE_GCCGO
	GOLDFLAGS = -ldflags '-s -w'
else
	GOLDFLAGS = -s -w  -Os -O2
endif

ifdef GODEBUG
	GOLDFLAGS =
endif

ifndef USE_GCCGO
	GOBUILD = go build ${GOLDFLAGS}
else
	GOLDFLAGS += $(shell pkg-config --libs gio-2.0 gtk+-3.0 gdk-pixbuf-xlib-2.0 x11 libcanberra xi)
	GOBUILD = go build -compiler gccgo -gccgoflags "${GOLDFLAGS}"
endif

all: build

prepare:
	@if [ ! -d ${GOPATH_DIR}/src/${GOPKG_PREFIX} ]; then \
		mkdir -p ${GOPATH_DIR}/src/$(dir ${GOPKG_PREFIX}); \
		ln -sf ../../../.. ${GOPATH_DIR}/src/${GOPKG_PREFIX}; \
		fi

startdde:
	env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -v -o startdde

dde-readahead/dde-readahead:
	cd dde-readahead; env GOPATH="${CURDIR}/${GOPATH_DIR}:${GOPATH}" ${GOBUILD} -v -o dde-readahead

build: prepare startdde dde-readahead/dde-readahead

install:
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions
	@for i in $(shell ls misc/xsessions/ | grep -E '*.in$$' );do sed 's|@PREFIX@|$(PREFIX)|g' misc/xsessions/$$i > ${DESTDIR}${PREFIX}/share/xsessions/$${i%.in}; done
	install -Dm755 dde-readahead/dde-readahead ${DESTDIR}/${PREFIX}/lib/deepin-daemon/dde-readahead
	install -Dm644 dde-readahead/dde-readahead.service ${DESTDIR}/lib/systemd/system/dde-readahead.service
	mkdir -p ${DESTDIR}/lib/systemd/system/multi-user.target.wants/
	ln -s /lib/systemd/system/dde-readahead.service ${DESTDIR}/lib/systemd/system/multi-user.target.wants/dde-readahead.service

clean:
	-rm -rf ${GOPATH_DIR}
	-rm -f startdde
	-rm -f dde-readahead/dde-readahead

rebuild: clean build
