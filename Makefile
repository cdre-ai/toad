build:
	go build -o toad .

test:
	go test -race ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f toad
	rm -rf dist/

.PHONY: build test lint vet fmt clean
