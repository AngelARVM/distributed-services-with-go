PROTOC_GEN_GO := $(CURDIR)/bin/protoc-gen-go

.PHONY: compile test dev

compile: $(PROTOC_GEN_GO)
	PATH="$(CURDIR)/bin:$$PATH" protoc api/v1/*.proto \
		--go_out=. \
		--go_opt=paths=source_relative \
		--proto_path=.

$(PROTOC_GEN_GO):
	GOBIN=$(CURDIR)/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.25.0

test:
	go test -race ./...

dev:
	air -c .air.toml
