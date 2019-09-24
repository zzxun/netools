# Makefile for coredns customized plugins
all: linter test

.PHONY: test
test:
	GO111MODULE=on go test -v ./... -test.bench=".*" --tags etcd -benchmem -cover

.PHONY: linter
linter:
	GO111MODULE=on gometalinter --deadline=2m --disable-all --enable=golint --enable=vet --vendor --exclude=^pb/ ./...

.PHONY: clean
clean:
	GO111MODULE=on go clean

