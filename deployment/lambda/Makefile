help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

packages := $(shell find . -name \*main.go -not -path "./node_modules/*" | awk -F'/' '{print $$2}')
pwd := ${CURDIR}
git_root := $(shell git rev-parse --show-toplevel)

build: clean
	@for package in $(packages) ; do \
  		set -e; \
		echo Build $(pwd)/bin/$$package ; \
		env GOOS=linux go build -ldflags="-s -w" -gcflags=all=-trimpath=$(git_root) -asmflags=all=-trimpath=$(git_root) -o bin/$$package ./$$package/ ; \
	done

build-local: clean
	@for package in $(packages) ; do \
  		set -e; \
		echo Build $(pwd)/bin/$$package ; \
		env go build -ldflags="-s -w" -gcflags=all=-trimpath=$(git_root) -asmflags=all=-trimpath=$(git_root) -o bin/$$package ./$$package/ ; \
	done

install:
	npm install

clean:
	rm -rf ./bin

