BINARY_NAME=authifi
VERSION=$(shell git describe --tags --abbrev=0 || echo "undefined")

all: lint build test
 
build:
	go build -ldflags="-X 'main.version=${VERSION}'" -o ${BINARY_NAME} main.go
 
test:
	go test -v ./...
 
run:
	go run main.go serve -c config.cfg

clean:
	go clean
	rm ${BINARY_NAME}

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
