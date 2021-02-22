SERVICE_NAME := ilert-kube-agent
PROJECT_PACKAGE := github.com/iLert/ilert-kube-agent

build: clean
	env CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/$(SERVICE_NAME) ./cmd/

build-local: clean
	env CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(SERVICE_NAME) ./cmd/

clean:
	rm -rf ./bin

run:
	go run ./cmd/

code-generator:
	docker run -it --rm \
		-v ${PWD}:/go/src/$(PROJECT_PACKAGE)\
		-e PROJECT_PACKAGE=$(PROJECT_PACKAGE) \
		-e CLIENT_GENERATOR_OUT=$(PROJECT_PACKAGE)/pkg/client \
		-e APIS_ROOT=$(PROJECT_PACKAGE)/pkg/apis \
		-e GROUPS_VERSION="incident:v1" \
		-e GENERATION_TARGETS="deepcopy,client" \
		quay.io/slok/kube-code-generator:v1.17.3
