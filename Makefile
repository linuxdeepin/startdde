
PREFIX = /usr

all: build

build:
	echo "TODO"

install:
	echo "TODO"

test-splash-prepare:
	ln -vf misc/splash_dev_test.go .

test-splash-clean:
	rm -f splash_dev_test.go

test-splash: test-splash-prepare
	go test -gocheck.v -gocheck.f TestSplash

test-splash-read-root-prop: test-splash-prepare
	go test -gocheck.v -gocheck.f TestSplashReadRootProp

clean: test-splash-clean
	echo "TODO"

rebuild: clean build
