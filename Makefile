BUILD_TIME := $(shell date --rfc-3339=seconds)
COMMIT_ID := $(shell git rev-parse HEAD)
LSP_PKG := $(shell go list -f '{{.ImportPath}}' ./lsp)
CMD_PKG := $(shell go list -f '{{.ImportPath}}' ./cmd)

LDFLAGS = -X "$(LSP_PKG).BuildTime='"$(BUILD_TIME)"'" -X "$(LSP_PKG).CommitId='"$(COMMIT_ID)"'"

SRC := $(shell find . -type f -name '*.go') lsp/template/default/*
PROTO := $(shell find . -type f -name '*.proto')
COV := .coverage.out
TARGET := DDBOT

$(COV): $(SRC)
	go test ./... -coverprofile=$(COV)


$(TARGET): $(SRC) go.mod go.sum
	go build -ldflags '$(LDFLAGS)' -o $(TARGET) $(CMD_PKG)

build: $(TARGET)

proto: $(PROTO)
	protoc --go_out=. $(PROTO)

test: $(COV)

coverage: $(COV)
	go tool cover -func=$(COV) | grep -v 'pb.go'

report: $(COV)
	go tool cover -html=$(COV)

clean:
	- rm -rf $(TARGET) $(COV)
