run:
	@go run cmd/main.go

tidy:
	@go mod tidy

vendor:
	tidy;
	@go mod vendor

gen:
	@go generate ./...

fmt:
	@go fmt ./...
	@golangci-lint fmt

lint: fmt
	@golangci-lint run --build-tags=integration

test:
	@go test ./...

it: 
	@go test -tags=integration ./integration

.PHONY: run tidy vendor gen fmt lint test it