PROTOC_GEN_GO := $(CURDIR)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC := $(CURDIR)/bin/protoc-gen-go-grpc
CFSSL := $(CURDIR)/bin/cfssl
CFSSLJSON := $(CURDIR)/bin/cfssljson

.PHONY: compile test dev generate-tools generate-cert-tools init gencert gencerts

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

CONFIG_PATH=${HOME}/.prolog/

.PHONY: init
init:
	mkdir -p ${CONFIG_PATH}

generate-cert-tools: $(CFSSL) $(CFSSLJSON)

$(CFSSL):
	go build -o $(CFSSL) github.com/cloudflare/cfssl/cmd/cfssl

$(CFSSLJSON):
	go build -o $(CFSSLJSON) github.com/cloudflare/cfssl/cmd/cfssljson

gencert: generate-cert-tools
	$(CFSSL) gencert \
		-initca test/ca-csr.json | $(CFSSLJSON) -bare ca
	$(CFSSL) gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=server \
		test/server-csr.json | $(CFSSLJSON) -bare server
	$(CFSSL) gencert \
	  -ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="root" \
		test/client-csr.json | $(CFSSLJSON) -bare root-client

	$(CFSSL) gencert \
	  -ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=test/ca-config.json \
		-profile=client \
		-cn="nobody" \
		test/client-csr.json | $(CFSSLJSON) -bare nobody-client

gencerts: gencert

$(CONFIG_PATH)/model.conf:
	cp test/model.conf $(CONFIG_PATH)/model.conf


$(CONFIG_PATH)/policy.csv:
	cp test/policy.csv $(CONFIG_PATH)/policy.csv
