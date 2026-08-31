#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RAW_VERSION="${1:-${VERSION:-}}"

fail() {
	echo "Release failed: $*" >&2
	exit 1
}

usage() {
	echo "Usage: $0 <version>" >&2
	echo "Example: $0 0.2.0" >&2
}

if [[ -z "$RAW_VERSION" ]]; then
	usage
	exit 1
fi

VERSION="${RAW_VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	fail "version must use the 0.2.0 format"
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
	fail "releases must be built and published from macOS"
fi

for tool in git gh shasum; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		fail "missing required tool: $tool"
	fi
done

cd "$PROJECT_DIR"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	fail "the project directory is not a Git repository"
fi
if ! git diff --quiet || ! git diff --cached --quiet || [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
	fail "the Git working tree must be clean"
fi

if ! gh auth status >/dev/null 2>&1; then
	fail "GitHub CLI is not authenticated; run: gh auth login"
fi

REPOSITORY="${GITHUB_REPOSITORY:-}"
if [[ -z "$REPOSITORY" ]]; then
	REPOSITORY="$(gh repo view --json nameWithOwner --jq '.nameWithOwner')"
fi
if [[ ! "$REPOSITORY" =~ ^[^/]+/[^/]+$ ]]; then
	fail "could not determine the GitHub repository"
fi

CURRENT_BRANCH="$(git branch --show-current)"
if [[ -z "$CURRENT_BRANCH" ]]; then
	fail "releases cannot be published from a detached HEAD"
fi
git fetch origin --quiet
if ! git rev-parse --verify "origin/$CURRENT_BRANCH" >/dev/null 2>&1; then
	fail "remote branch origin/$CURRENT_BRANCH does not exist"
fi

HEAD_SHA="$(git rev-parse HEAD)"
REMOTE_SHA="$(git rev-parse "origin/$CURRENT_BRANCH")"
if [[ "$HEAD_SHA" != "$REMOTE_SHA" ]]; then
	fail "local $CURRENT_BRANCH must exactly match origin/$CURRENT_BRANCH; push or pull first"
fi
if ! gh api "repos/$REPOSITORY/commits/$HEAD_SHA" >/dev/null 2>&1; then
	fail "current commit $HEAD_SHA is not available in $REPOSITORY"
fi

TAG="v$VERSION"
for existing_tag in "$TAG" "$VERSION"; do
	if git show-ref --verify --quiet "refs/tags/$existing_tag"; then
		fail "local tag $existing_tag already exists"
	fi
done

ensure_github_resource_absent() {
	local label="$1"
	local endpoint="$2"
	local response
	if response="$(gh api "$endpoint" 2>&1)"; then
		fail "$label already exists"
	fi
	if [[ "$response" != *"HTTP 404"* ]]; then
		echo "$response" >&2
		fail "could not verify whether $label exists"
	fi
}

for existing_tag in "$TAG" "$VERSION"; do
	ensure_github_resource_absent "tag $existing_tag" "repos/$REPOSITORY/git/ref/tags/$existing_tag"
	ensure_github_resource_absent "release $existing_tag" "repos/$REPOSITORY/releases/tags/$existing_tag"
done

BUILD_NUMBER="${BUILD_NUMBER:-$(date -u +%Y%m%d%H%M%S)}"
APP_NAME="AIQuota"
ASSET_PATH="$PROJECT_DIR/dist/$APP_NAME-$VERSION-macos.zip"
CHECKSUM_PATH="$PROJECT_DIR/dist/SHA256SUMS"

echo "→ Building $TAG from $HEAD_SHA"
VERSION="$VERSION" BUILD_NUMBER="$BUILD_NUMBER" "$PROJECT_DIR/scripts/build.sh"

if [[ ! -f "$ASSET_PATH" ]]; then
	fail "build completed without producing $(basename "$ASSET_PATH")"
fi

(
	cd "$PROJECT_DIR/dist"
	shasum -a 256 "$(basename "$ASSET_PATH")" > "$(basename "$CHECKSUM_PATH")"
)

echo "→ Publishing $TAG to $REPOSITORY"
gh release create "$TAG" \
	"$ASSET_PATH#AI quota $VERSION for macOS" \
	"$CHECKSUM_PATH#SHA-256 checksums" \
	--repo "$REPOSITORY" \
	--target "$HEAD_SHA" \
	--title "AI quota $VERSION" \
	--generate-notes \
	--latest

RELEASE_URL="$(gh release view "$TAG" --repo "$REPOSITORY" --json url --jq '.url')"
echo
echo "Published: $RELEASE_URL"
