.PHONY: build test vet web-test clean

VERSION ?= dev

build:
	go build -ldflags "-s -w -X github.com/originalpages/r2m/internal/cli.Version=$(VERSION)" -o r2m ./cmd/r2m

test:
	go vet ./...
	go test -race ./...

web-test:
	cd web && npm ci && npm test && npm run build

clean:
	rm -f r2m r2m.exe
