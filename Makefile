
export GO111MODULE=on

PACKAGES ?= $(shell go list -mod=vendor ./... | grep -v /vendor/)

all: build

build:
	go build -mod=vendor

clean:
	go clean -i ./...
	@rm -rf bin
	@rm -f *.rdb *.out *.log *.dump 
	@if [ -d test ]; then cd test && rm -f *.out *.log *.rdb; fi

fmt:
	gofmt -w -s  . 2>&1 | grep -vE 'vendor' | awk '{print} END{if(NR>0) {exit 1}}'

vet:
	go vet -mod=vendor $(PACKAGES)

test:
	go test -mod=vendor -race $(PACKAGES)
