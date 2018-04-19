SHELL := $(shell which bash)
OSARCH := "linux/amd64 linux/386 windows/amd64 windows/386 darwin/amd64 darwin/386"
TAG ?= master
DOCKER_REPO ?= thomasjpfan/docker-scaler

.SHELLFLAGS = -c
.ONESHELL: ;
.NOTPARALLEL: ;

.PHONY: all

dep:
	go get -v -u github.com/golang/dep/cmd/dep && \
	go get github.com/mattn/goveralls

build_image:
	docker image build -t $(DOCKER_REPO):$(TAG) .

unit_test:
	go test ./... --run UnitTest

deploy_test:
	docker stack deploy -c stacks/docker-scaler-test.yml test

undeploy_test:
	docker stack rm test

integration_test:
	./scripts/integration_test.sh

cross-build: ## Build the app for multiple os/arch
    gox -osarch=$(OSARCH) -output "bin/docker-scaler_{{.OS}}_{{.Arch}}"
