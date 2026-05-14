.PHONY: test
test:
	go test -v -race ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: tools
tools:
	go install github.com/yoheimuta/protolint/cmd/protolint@latest
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.12.0
	@echo "checking protobuf compiler, if it fails follow guide at https://protobuf.dev/installation/"
	@which -s protoc && echo OK || exit 1

.PHONY: protolint
protolint:
	protolint .

.PHONY: protobuf
protobuf:
	protoc --go_out=. --go_opt=paths=source_relative \
               --go-grpc_out=. --go-grpc_opt=paths=source_relative \
               proto/dns/v1/dns.proto