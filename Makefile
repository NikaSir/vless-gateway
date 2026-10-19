GO ?= go
VERSION := $(shell tr -d '\n' < VERSION)
LDFLAGS := -s -w -buildid= -X github.com/NikaSir/vless-gateway/internal/buildinfo.Version=$(VERSION)
.PHONY: fmt vet test test-race build-arm64 verify clean
fmt:
	$(GO) fmt ./...
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...
test-race:
	$(GO) test -race ./...
build-arm64:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/gatewayd ./cmd/gatewayd
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o dist/gatewayctl ./cmd/gatewayctl
verify: vet test test-race
clean:
	$(GO) clean ./...
