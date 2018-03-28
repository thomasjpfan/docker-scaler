.PHONY: build unit_test deploy_test integration_test undeploy_test

TAG?=master

build:
	docker image build -t thomasjpfan/docker-scaler:$(TAG) .

unit_test:
	go test ./... --run UnitTest

deploy_test:
	docker stack deploy -c stacks/docker-scaler-test.yml test

undeploy_test:
	docker stack rm test

integration_test:
	./scripts/integration_test.sh
