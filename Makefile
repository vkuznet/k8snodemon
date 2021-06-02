#GOPATH:=$(PWD):${GOPATH}
#export GOPATH
OS := $(shell uname)
ifeq ($(OS),Darwin)
flags=-ldflags="-s -w"
else
flags=-ldflags="-s -w -extldflags -static"
endif
TAG := $(shell git tag | sort -r | head -n 1)

all: build

build:
	go clean; rm -rf pkg k8snodemon*; go build ${flags}

build_all: build_osx build_linux build_power8 build_arm64 build_windows

build_osx:
	go clean; rm -rf pkg k8snodemon_osx; GOOS=darwin go build ${flags}
	mv k8snodemon k8snodemon_osx

build_linux:
	go clean; rm -rf pkg k8snodemon_amd64; CGO_ENABLED=0 GOOS=linux go build ${flags}
	mv k8snodemon k8snodemon_amd64

build_power8:
	go clean; rm -rf pkg k8snodemon_ppc64le; GOARCH=ppc64le GOOS=linux go build ${flags}
	mv k8snodemon k8snodemon_ppc64le

build_arm64:
	go clean; rm -rf pkg k8snodemon_aarch64; GOARCH=arm64 GOOS=linux go build ${flags}
	mv k8snodemon k8snodemon_aarch64

build_windows:
	go clean; rm -rf pkg k8snodemon.exe; GOARCH=amd64 GOOS=windows go build ${flags}

install:
	go install

clean:
	go clean; rm -rf pkg

test : test1

test1:
	go test
