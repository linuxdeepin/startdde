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
	GOLDFLAGS += $(shell pkg-config --libs gio-2.0 gtk+-3.0 gdk-pixbuf-xlib-2.0 x11 xcursor xfixes libcanberra xi)
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

build: prepare startdde

install:
	install -Dm755 startdde ${DESTDIR}${PREFIX}/bin/startdde
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions
	@for i in $(shell ls misc/xsessions/ | grep -E '*.in$$' );do sed 's|@PREFIX@|$(PREFIX)|g' misc/xsessions/$$i > ${DESTDIR}${PREFIX}/share/xsessions/$${i%.in}; done
	mkdir -p ${DESTDIR}${PREFIX}/share/startdde/
	cp -f misc/config/* ${DESTDIR}${PREFIX}/share/startdde/

clean:
	-rm -rf ${GOPATH_DIR}
	-rm -f startdde

rebuild: clean build
