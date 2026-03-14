#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8090}"
SIZES_MB="${SIZES_MB:-10 100 500 1000}"
TMPDIR="${TMPDIR:-/tmp}"
PREFIX="perf-test"
STCTL="${STCTL:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${SCRIPT_DIR}/../bin"

# Use stctl if --stctl flag is passed or STCTL env var is set
if [[ "${1:-}" == "--stctl" ]]; then
    STCTL="${BIN_DIR}/stctl"
    shift
elif [[ -n "$STCTL" ]]; then
    : # use whatever STCTL is set to
fi

if [[ -n "$STCTL" ]] && [[ ! -x "$STCTL" ]]; then
    echo "Error: stctl not found at $STCTL (run 'make build-stctl')"
    exit 1
fi

cleanup() {
    for size in $SIZES_MB; do
        rm -f "$TMPDIR/test-${size}m.bin"
    done
    if [[ -n "$STCTL" ]]; then
        "$STCTL" -s "$BASE_URL" dir rm "$PREFIX" > /dev/null 2>&1 || true
    else
        curl -s -X DELETE "$BASE_URL/dirs/$PREFIX" > /dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

fmt_speed() {
    local bytes=$1
    local secs=$2
    local mb_per_sec
    mb_per_sec=$(awk "BEGIN {printf \"%.1f\", $bytes / $secs / 1048576}")
    echo "$mb_per_sec MB/s"
}

upload_curl() {
    local size=$1
    curl -s -o /dev/null -w "%{time_total} %{speed_upload}" \
        -X PUT -F "file=@$TMPDIR/test-${size}m.bin" \
        "$BASE_URL/files/$PREFIX/test-${size}m.bin"
}

upload_stctl() {
    local size=$1
    local file_bytes
    file_bytes=$(stat -c%s "$TMPDIR/test-${size}m.bin")
    local start end elapsed
    start=$(date +%s%N)
    "$STCTL" -s "$BASE_URL" file put "$PREFIX/test-${size}m.bin" "$TMPDIR/test-${size}m.bin" > /dev/null
    end=$(date +%s%N)
    elapsed=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
    echo "$elapsed $file_bytes"
}

download_curl() {
    local size=$1
    curl -s -o /dev/null -w "%{time_total} %{speed_download}" \
        "$BASE_URL/files/$PREFIX/test-${size}m.bin"
}

download_stctl() {
    local size=$1
    local file_bytes
    file_bytes=$(stat -c%s "$TMPDIR/test-${size}m.bin")
    local start end elapsed
    start=$(date +%s%N)
    "$STCTL" -s "$BASE_URL" file get "$PREFIX/test-${size}m.bin" /dev/null > /dev/null
    end=$(date +%s%N)
    elapsed=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
    echo "$elapsed $file_bytes"
}

MODE="curl"
if [[ -n "$STCTL" ]]; then
    MODE="stctl ($(basename "$STCTL"))"
fi

echo "=== ha-store upload benchmark ==="
echo "Server: $BASE_URL"
echo "Client: $MODE"
echo ""

# Generate test files
echo "Generating test files..."
for size in $SIZES_MB; do
    dd if=/dev/urandom of="$TMPDIR/test-${size}m.bin" bs=1M count="$size" 2>/dev/null
done
echo ""

# Upload tests
echo "Upload:"
printf "%-10s %-12s %-15s\n" "Size" "Time" "Throughput"
printf "%-10s %-12s %-15s\n" "----" "----" "----------"

for size in $SIZES_MB; do
    if [[ -n "$STCTL" ]]; then
        result=$(upload_stctl "$size")
        time_total=$(echo "$result" | awk '{print $1}')
        file_bytes=$(echo "$result" | awk '{print $2}')
        printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$(fmt_speed "$file_bytes" "$time_total")"
    else
        result=$(upload_curl "$size")
        time_total=$(echo "$result" | awk '{print $1}')
        speed=$(echo "$result" | awk '{print $2}')
        mb_per_sec=$(awk "BEGIN {printf \"%.1f\", $speed / 1048576}")
        printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$mb_per_sec MB/s"
    fi
done

echo ""

# Download tests
echo "Download:"
printf "%-10s %-12s %-15s\n" "Size" "Time" "Throughput"
printf "%-10s %-12s %-15s\n" "----" "----" "----------"

for size in $SIZES_MB; do
    if [[ -n "$STCTL" ]]; then
        result=$(download_stctl "$size")
        time_total=$(echo "$result" | awk '{print $1}')
        file_bytes=$(echo "$result" | awk '{print $2}')
        printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$(fmt_speed "$file_bytes" "$time_total")"
    else
        result=$(download_curl "$size")
        time_total=$(echo "$result" | awk '{print $1}')
        speed=$(echo "$result" | awk '{print $2}')
        mb_per_sec=$(awk "BEGIN {printf \"%.1f\", $speed / 1048576}")
        printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$mb_per_sec MB/s"
    fi
done
