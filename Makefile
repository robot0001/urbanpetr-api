.PHONY: build test lint clean

build:
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bootstrap ./cmd/api
	zip -j lambda.zip bootstrap

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

clean:
	rm -f bootstrap lambda.zip
