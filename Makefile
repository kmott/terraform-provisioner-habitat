TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=github.com
NAMESPACE=kmott
NAME=habitat
BINARY=terraform-provisioner-${NAME}
VERSION=0.1.4
OS_ARCH=linux_amd64

default: install

all: clean test-acceptance install

lint:
	golangci-lint run habitat/...

build: test-acceptance lint
	goreleaser build --snapshot --rm-dist --config=.goreleaser/.goreleaser.yml

install: build
	mkdir -p ~/.terraform.d/plugins
	cp dist/${BINARY}_${OS_ARCH}/${BINARY} ~/.terraform.d/plugins/${BINARY}_v${VERSION}

release:
	@git tag -a v${VERSION} -m "Tag v${VERSION}"
	@git push origin v${VERSION}
	@goreleaser --rm-dist --config=.goreleaser/.goreleaser.yml

test: test-acceptance test-integration

test-acceptance:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 5m

test-integration:
	$(MAKE) -C test all

test-integration-cleanup:
	$(MAKE) -C test/terraform destroy

clean:
	-rm ~/.terraform.d/plugins/${BINARY}_*

.PHONY: default all build install release test test-acceptance test-integration test-integration-cleanup clean
