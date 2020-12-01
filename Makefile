TEST?=$$(go list ./... | grep -v 'vendor')
HOSTNAME=github.com
NAMESPACE=kmott
NAME=habitat
BINARY=terraform-provisioner-${NAME}
VERSION=0.0.2
OS_ARCH=linux_amd64

default: install

all: clean test-acceptance install

lint:
	golangci-lint run habitat/...

build: lint
	goreleaser build --snapshot --rm-dist --config=.goreleaser/.goreleaser.yml

install: build
	mkdir -p ~/.terraform.d/plugins
	cp dist/${BINARY}_${OS_ARCH}/${BINARY} ~/.terraform.d/plugins/${BINARY}_v${VERSION}

release: test
	goreleaser release --snapshot --rm-dist --config=.build/.goreleaser.yml

test-acceptance:
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m

test-integration: all
	$(MAKE) -C test/terraform apply
	@echo "Waiting ~5 minutes for effortless convergence ..."
	@sleep 300
	$(MAKE) -C test/inspec test

test-integration-cleanup:
	$(MAKE) -C test/terraform destroy

clean:
	-rm ~/.terraform.d/plugins/${BINARY}_*

.PHONY: default all build install release test-acceptance test-integration test-integration-cleanup clean
