HASH := $(shell git rev-parse --short HEAD)

.PHONY: all
all: build

.PHONY: build
build:
	go build -o monitor

.PHONY: docker
docker:
	docker build -t timmydo/external-monitor:git-$(HASH) .
