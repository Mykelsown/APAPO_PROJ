# Apapo backend tasks. Go code lives in backend/, Docker files at the repo root.
IMAGE ?= apapo-api
VERSION ?= dev

.PHONY: run test build docker up down fmt vet check hooks

run:
	cd backend && go run ./cmd/api

test:
	cd backend && go test -race ./...

fmt:
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

vet:
	cd backend && go vet ./...

check: fmt vet test

build:
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.buildVersion=$(VERSION)" -o bin/apapo-api ./cmd/api

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

up:
	docker compose up --build

down:
	docker compose down

hooks:
	cp scripts/commit-msg-hook.sh .git/hooks/commit-msg && chmod +x .git/hooks/commit-msg
