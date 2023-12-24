.PHONY: build test glua

build:
	./_tools/go-inline *.go && go fmt . &&  go build

glua: *.go pm/*.go cmd/glua/glua.go
	./_tools/go-inline *.go && go fmt . && go build cmd/glua/glua.go

test:
	./_tools/go-inline *.go && go fmt . &&  go test

migrate-lvalue:
	python3 ./_tools/migrate-lvalue.py *.go && go fmt .
	python3 ./_tools/migrate-lvalue2.py *.go && go fmt .