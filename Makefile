.PHONY: update-tools install lint check format dev dependencies down test update-license-token

SHELL := /bin/bash

GO := true
JUNO_CI_CLONE_PATH=.juno-ci
GOBIN ?= $$(go env GOPATH)/bin


# updater
update-tools:
	@ echo " >> Pulling Latest Tools << "
	@ rm -rf Development-Tools
	@ git clone https://github.com/juno-fx/Development-Tools.git
	@ rm -rf .tools
	@ mv -v Development-Tools/.tools .tools
	@ rm -rf Development-Tools
	@ echo " >> Tools Updated << "

.tools/cluster.Makefile:
	@ $(MAKE) update-tools

.tools/dev.Makefile:
	@ $(MAKE) update-tools

# Environment targets
dev: .tools/cluster.Makefile
	@ $(MAKE) -f .tools/cluster.Makefile dev --no-print-directory

down: .tools/cluster.Makefile
	@ $(MAKE) -f .tools/cluster.Makefile down --no-print-directory

dependencies: .tools/cluster.Makefile
	@kubectl apply -f crds

unit:
	@ echo " >> Running Go unit tests << "
	go test -gcflags 'all=-N -l' ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./...

coverage: unit
	${GOBIN}/go-test-coverage --config=./.testcoverage.yaml
	
coverage-html: unit
	go tool cover -html=cover.out -o cover.html
	${GOBIN}/go-test-coverage --config=./.testcoverage.yaml || true
	xdg-open cover.html

test: coverage 

integration:
	@ echo " >> Running e2e tests << "
	@ $(MAKE) -f .tools/cluster.Makefile test $(ENV) --no-print-directory

install:
	@ # This is to pass CI for now

check:
	@ $(MAKE) lint

lint:
	devbox run golangci-lint fmt
	devbox run golangci-lint run