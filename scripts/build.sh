#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
BUILD_NUMBER="${BUILD_NUMBER:-1}"
APP_NAME="AIQuota"
APP_BUNDLE="$PROJECT_DIR/dist/$APP_NAME.app"
ZIP_FILE="$PROJECT_DIR/dist/$APP_NAME-$VERSION-macos.zip"
ICONSET_DIR="$PROJECT_DIR/build/$APP_NAME.iconset"

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	echo "VERSION must use the 0.1.0 format." >&2
	exit 1
fi
if [[ ! "$BUILD_NUMBER" =~ ^[0-9]+$ ]]; then
	echo "BUILD_NUMBER must be a non-negative integer." >&2
	exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "The .app packaging script must run on macOS." >&2
	exit 1
fi

for tool in go iconutil codesign ditto; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "Missing required tool: $tool" >&2
		exit 1
	fi
done

mkdir -p "$PROJECT_DIR/build" "$PROJECT_DIR/dist"
rm -rf "$APP_BUNDLE" "$ICONSET_DIR"
mkdir -p "$APP_BUNDLE/Contents/MacOS" "$APP_BUNDLE/Contents/Resources" "$ICONSET_DIR"

cd "$PROJECT_DIR"
echo "→ Downloading dependencies"
go mod download

echo "→ Running tests"
go test ./...

echo "→ Generating the app icon"
go run ./cmd/icon-gen -out "$ICONSET_DIR"
iconutil -c icns "$ICONSET_DIR" -o "$APP_BUNDLE/Contents/Resources/$APP_NAME.icns"

echo "→ Building AI quota $VERSION"
CGO_ENABLED=1 go build \
	-trimpath \
	-ldflags "-s -w -X main.version=$VERSION" \
	-o "$APP_BUNDLE/Contents/MacOS/$APP_NAME" \
	./cmd/aiquota

sed \
	-e "s/__VERSION__/$VERSION/g" \
	-e "s/__BUILD_NUMBER__/$BUILD_NUMBER/g" \
	"$PROJECT_DIR/packaging/macos/Info.plist" > "$APP_BUNDLE/Contents/Info.plist"

SIGN_IDENTITY="${CODESIGN_IDENTITY:--}"
echo "→ Signing the app with identity: $SIGN_IDENTITY"
codesign --force --deep --sign "$SIGN_IDENTITY" "$APP_BUNDLE"
codesign --verify --deep --strict "$APP_BUNDLE"

rm -f "$ZIP_FILE"
ditto -c -k --sequesterRsrc --keepParent "$APP_BUNDLE" "$ZIP_FILE"

echo
echo "Created:"
echo "  $APP_BUNDLE"
echo "  $ZIP_FILE"
echo
echo "Run with: open \"$APP_BUNDLE\""
