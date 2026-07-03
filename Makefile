PROTOC_GEN_GO := $(CURDIR)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(CURDIR)/bin/protoc-gen-go-grpc

.PHONY: compile test dev generate-tools

compile: generate-tools
	protoc api/v1/*.proto \
	--plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
	--plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
	--go_out=. \
	--go-grpc_out=. \
	--go_opt=paths=source_relative \
	--go-grpc_opt=paths=source_relative \
	--proto_path=.

generate-tools: $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)

$(PROTOC_GEN_GO):
	go build -o $(PROTOC_GEN_GO) google.golang.org/protobuf/cmd/protoc-gen-go

$(PROTOC_GEN_GO_GRPC):
	go build -o $(PROTOC_GEN_GO_GRPC) google.golang.org/grpc/cmd/protoc-gen-go-grpc


test:
	go test -race ./...

dev:
	air -c .air.toml
