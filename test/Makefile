export GOPATH ?= ${HOME}/go
export GOBIN  ?= ${GOPATH}/bin
export GOPKG  ?= ${GOPATH}/pkg
PATH          := $(PATH):${GOBIN}

all: preflight-check tidy vendor
	@ginkgo -failFast -v -timeout 30m ./integration

preflight-check:
	@if [ ! -e "${GOBIN}/ginkgo" ] || [ ! -d "vendor" ]; then	\
		go get github.com/onsi/ginkgo/ginkgo;					\
		go mod vendor;											\
	fi

tidy:
	@go mod tidy

vendor:
	@go mod vendor

.PHONY: all preflight-check tidy vendor
