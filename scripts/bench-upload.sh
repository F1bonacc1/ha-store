#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8090}"
SIZES_MB="${SIZES_MB:-10 100 500 1000}"
TMPDIR="${TMPDIR:-/tmp}"
PREFIX="perf-test"

cleanup() {
    for size in $SIZES_MB; do
        rm -f "$TMPDIR/test-${size}m.bin"
    done
    curl -s -X DELETE "$BASE_URL/dirs/$PREFIX" > /dev/null 2>&1 || true
}
trap cleanup EXIT

fmt_speed() {
    local bytes_per_sec=$1
    local mb_per_sec
    mb_per_sec=$(awk "BEGIN {printf \"%.1f\", $bytes_per_sec / 1048576}")
    echo "$mb_per_sec MB/s"
}

echo "=== ha-store upload benchmark ==="
echo "Server: $BASE_URL"
echo ""

# Generate test files
echo "Generating test files..."
for size in $SIZES_MB; do
    dd if=/dev/urandom of="$TMPDIR/test-${size}m.bin" bs=1M count="$size" 2>/dev/null
done
echo ""

# Upload tests
printf "%-10s %-12s %-15s\n" "Size" "Time" "Throughput"
printf "%-10s %-12s %-15s\n" "----" "----" "----------"

for size in $SIZES_MB; do
    result=$(curl -s -o /dev/null -w "%{time_total} %{speed_upload}" \
        -X PUT -F "file=@$TMPDIR/test-${size}m.bin" \
        "$BASE_URL/files/$PREFIX/test-${size}m.bin")
    time_total=$(echo "$result" | awk '{print $1}')
    speed=$(echo "$result" | awk '{print $2}')
    printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$(fmt_speed "$speed")"
done

echo ""

# Download tests
printf "%-10s %-12s %-15s\n" "Size" "Time" "Throughput"
printf "%-10s %-12s %-15s\n" "----" "----" "----------"

for size in $SIZES_MB; do
    result=$(curl -s -o /dev/null -w "%{time_total} %{speed_download}" \
        "$BASE_URL/files/$PREFIX/test-${size}m.bin")
    time_total=$(echo "$result" | awk '{print $1}')
    speed=$(echo "$result" | awk '{print $2}')
    printf "%-10s %-12s %-15s\n" "${size}MB" "${time_total}s" "$(fmt_speed "$speed")"
done
