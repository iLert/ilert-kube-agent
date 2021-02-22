service_name := ilert-kube-agent

build: clean
	env CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o bin/$(service_name) ./cmd/

build-local: clean
	env CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(service_name) ./cmd/

clean:
	rm -rf ./bin

run:
	go run ./cmd/
