.PHONY: test run build release clean

test:
	go test ./...

run:
	CGO_ENABLED=1 go run ./cmd/aiquota

build:
	./scripts/build.sh

release:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required, for example: make release VERSION=0.2.0" >&2; exit 1; fi
	./scripts/release.sh "$(VERSION)"

clean:
	go clean
	@echo "You can remove build/ and dist/ manually when they are no longer needed."
