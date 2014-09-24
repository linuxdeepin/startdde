PREFIX = /usr

ifndef USE_GCCGO
    GOBUILD = go build
else
    LDFLAGS = $(shell pkg-config --libs gio-2.0 gdk-3.0 gdk-pixbuf-xlib-2.0 x11)
    GOBUILD = go build -compiler gccgo -gccgoflags "${LDFLAGS}"
endif

all: build

build:
	GOPATH=/usr/share/gocode ${GOBUILD} -o startdde
	echo "Start Building dialogUI"
	cd dialogUI && mkdir build
	cd dialogUI/build && cmake .. -DCMAKE_INSTALL_PREFIX=${PREFIX} -DCMAKE_BUILD_TYPE=Release

install:
	mkdir -p ${DESTDIR}${PREFIX}/bin && cp startdde ${DESTDIR}${PREFIX}/bin
	mkdir -p ${DESTDIR}${PREFIX}/share/xsessions && cp misc/*.desktop ${DESTDIR}${PREFIX}/share/xsessions
	echo "Install dialogUI"
	cd dialogUI/build && make DESTDIR=${DESTDIR} install

clean:
	rm -rf dialogUI/build

rebuild: clean build
