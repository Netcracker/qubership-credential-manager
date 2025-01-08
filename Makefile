DOCKER_FILE := build/Dockerfile

ifndef TAG_ENV
override TAG_ENV = local
endif

ifndef DOCKER_NAMES
override DOCKER_NAMES = "ghcr.io/netcracker/qubership-credential-manager:${TAG_ENV}"
endif

sandbox-build: deps docker-build

all: sandbox-build docker-push

local: fmt deps docker-build

deps:
	go mod tidy
	GO111MODULE=on

update:
	go get -u ./...

fmt:
	gofmt -l -s -w .

compile:
	CGO_ENABLED=0 go build -o ./build/_output/bin/qubership-credential-manager \
				-gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/qubership-credential-manager


docker-build:
	$(foreach docker_tag,$(DOCKER_NAMES),docker build --file="${DOCKER_FILE}" --pull -t $(docker_tag) ./;)

docker-push:
	$(foreach docker_tag,$(DOCKER_NAMES),docker push $(docker_tag);)

clean:
	rm -rf build/_output

test:
	go test -v ./...
