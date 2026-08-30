.PHONY: test run build clean

test:
	go test ./...

run:
	CGO_ENABLED=1 go run ./cmd/aiquota

build:
	./scripts/build.sh

clean:
	go clean
	@echo "You can remove build/ and dist/ manually when they are no longer needed."
