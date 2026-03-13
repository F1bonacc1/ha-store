#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8090}"
TMPDIR="${TMPDIR:-/tmp}"
WORKDIR=$(mktemp -d "$TMPDIR/ha-store-tgz-test.XXXXXX")
PREFIX="tgz-test-$$"

cleanup() {
    rm -rf "$WORKDIR"
    curl -s -X DELETE "$BASE_URL/dirs/$PREFIX" > /dev/null 2>&1 || true
}
trap cleanup EXIT

echo "=== ha-store tgz extract test ==="
echo "Server: $BASE_URL"
echo "Workdir: $WORKDIR"
echo ""

# Create a source directory with various test files
SRC="$WORKDIR/src"
mkdir -p "$SRC/subdir/nested"

echo "hello world" > "$SRC/file1.txt"
echo "second file" > "$SRC/file2.txt"
dd if=/dev/urandom of="$SRC/binary.bin" bs=1K count=64 2>/dev/null
echo "nested content" > "$SRC/subdir/deep.txt"
echo "deeply nested" > "$SRC/subdir/nested/leaf.txt"
printf 'line1\nline2\nline3\n' > "$SRC/multiline.txt"

echo "Created source directory with test files:"
find "$SRC" -type f -printf "  %P (%s bytes)\n" | sort

# Create tgz archive
ARCHIVE="$WORKDIR/test.tgz"
tar -czf "$ARCHIVE" -C "$SRC" .
echo ""
echo "Created archive: $(stat --printf='%s' "$ARCHIVE") bytes"

# Upload with extract
echo ""
echo "Uploading and extracting tgz to /$PREFIX ..."
status=$(curl -s -o /dev/null -w "%{http_code}" \
    -X PUT -F "file=@$ARCHIVE" \
    "$BASE_URL/dirs/$PREFIX?extract=tgz")

if [ "$status" != "200" ]; then
    echo "FAIL: upload returned HTTP $status"
    exit 1
fi
echo "Upload OK (HTTP $status)"

# Download each file and compare
echo ""
echo "Comparing files..."
DOWNLOAD="$WORKDIR/download"
mkdir -p "$DOWNLOAD"

PASS=0
FAIL=0

while IFS= read -r rel_path; do
    mkdir -p "$DOWNLOAD/$(dirname "$rel_path")"
    http_code=$(curl -s -o "$DOWNLOAD/$rel_path" -w "%{http_code}" \
        "$BASE_URL/files/$PREFIX/$rel_path")

    if [ "$http_code" != "200" ]; then
        echo "  FAIL: $rel_path — HTTP $http_code"
        FAIL=$((FAIL + 1))
        continue
    fi

    if cmp -s "$SRC/$rel_path" "$DOWNLOAD/$rel_path"; then
        echo "  OK:   $rel_path"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $rel_path — content mismatch"
        FAIL=$((FAIL + 1))
    fi
done < <(find "$SRC" -type f -printf "%P\n" | sort)

echo ""
echo "Results: $PASS passed, $FAIL failed"

if [ "$FAIL" -ne 0 ]; then
    exit 1
fi
