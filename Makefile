.PHONY: build test glua migrate-lvalue pgo

build:
	./_tools/go-inline *.go && go fmt . &&  go build

glua: *.go pm/*.go cmd/glua/glua.go
	./_tools/go-inline *.go && go fmt . && go build cmd/glua/glua.go

test:
	./_tools/go-inline *.go && go fmt . &&  go test

migrate-lvalue:
	python3 ./_tools/migrate-lvalue.py *.go && go fmt .
	python3 ./_tools/migrate-lvalue2.py *.go && go fmt .

pgo:
	mkdir -p build
	go build -o build/glua cmd/glua/glua.go
	time ./build/glua -p build/nopgo.prof _glua-tests/count.lua
	go build -o build/glua.pgo -pgo=build/nopgo.prof cmd/glua/glua.go
	time ./build/glua.pgo -p build/pgo.prof _glua-tests/count.lua
	go tool pprof -http :33801 build/pgo.prof
