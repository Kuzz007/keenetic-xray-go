#!/bin/sh
# build-ipk.sh -- hand-rolled .ipk builder, fallback for when goreleaser's
# nfpm integration doesn't produce something real opkg accepts (M7 spike,
# not yet verified). The ipk format doesn't need dedicated tooling: it's
# an `ar` archive of debian-binary, control.tar.gz, and data.tar.gz.
#
# Usage: build-ipk.sh <version> <arch> <binary-path> <output.ipk>
# Example:
#   build-ipk.sh 0.1.0-1 aarch64-3.10 dist/keenetic-xray-linux-arm64 \
#     dist/keenetic-xray_0.1.0-1_aarch64-3.10.ipk
set -eu

if [ "$#" -ne 4 ]; then
    echo "usage: $0 <version> <arch> <binary-path> <output.ipk>" >&2
    exit 1
fi

PKG_NAME="keenetic-xray"
PKG_VERSION="$1"
PKG_ARCH="$2"
BINARY_PATH="$3"
OUTPUT="$4"

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

case "$OUTPUT" in
    /*) OUTPUT_ABS="$OUTPUT" ;;
    *) OUTPUT_ABS="$(pwd)/$OUTPUT" ;;
esac
BINARY_DIR="$(CDPATH= cd -- "$(dirname -- "$BINARY_PATH")" && pwd)"
BINARY_ABS="$BINARY_DIR/$(basename -- "$BINARY_PATH")"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/control"
sed \
    -e "s/{{VERSION}}/$PKG_VERSION/" \
    -e "s/{{ARCH}}/$PKG_ARCH/" \
    "$SCRIPT_DIR/ipk/control.tmpl" > "$WORK/control/control"
cp "$SCRIPT_DIR/ipk/postinst" "$WORK/control/postinst"
cp "$SCRIPT_DIR/ipk/prerm" "$WORK/control/prerm"
cp "$SCRIPT_DIR/ipk/conffiles" "$WORK/control/conffiles"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"
tar --owner=0 --group=0 --numeric-owner -czf "$WORK/control.tar.gz" \
    -C "$WORK/control" control postinst prerm conffiles

mkdir -p "$WORK/data/opt/sbin" "$WORK/data/opt/etc/init.d"
cp "$BINARY_ABS" "$WORK/data/opt/sbin/$PKG_NAME"
chmod 0755 "$WORK/data/opt/sbin/$PKG_NAME"
cp "$SCRIPT_DIR/init.d/S99keenetic-xray" "$WORK/data/opt/etc/init.d/S99keenetic-xray"
chmod 0755 "$WORK/data/opt/etc/init.d/S99keenetic-xray"
tar --owner=0 --group=0 --numeric-owner -czf "$WORK/data.tar.gz" \
    -C "$WORK/data" opt

echo "2.0" > "$WORK/debian-binary"

# Hand-rolled ar archive, deliberately not `ar rc`: GNU ar's default
# output terminates every member name with "/" (its SysV/GNU dialect,
# used even for short names that don't need the extended-name-table
# mechanism), e.g. "debian-binary/" rather than "debian-binary". `ar t`/
# `ar x` round-trip that fine -- GNU tooling understands its own dialect
# -- but real opkg's minimal ar-parser does not strip it, never finds a
# member literally named "debian-binary", and rejects the whole file as
# "Malformed package file". Confirmed directly against real opkg on
# actual router hardware (aarch64 Keenetic/Entware): building this same
# script's `ar rc` output failed to install with exactly that error;
# switching to this hand-written common/BSD-style ar format (no trailing
# "/") installs cleanly. Do not "simplify" this back to `ar rc`.
ar_header() {
    # name mtime uid gid mode(octal, as text) size, then the fixed 2-byte
    # terminator "`\n" (0x60 0x0A) -- together exactly 60 bytes, per the
    # common ar format every .deb/.ipk consumer expects.
    printf '%-16s%-12s%-6s%-6s%-8s%-10s`\n' "$1" "0" "0" "0" "644" "$2"
}

rm -f "$OUTPUT_ABS"
(
    cd "$WORK"
    {
        printf '!<arch>\n'
        for member in debian-binary control.tar.gz data.tar.gz; do
            size=$(wc -c < "$member")
            ar_header "$member" "$size"
            cat "$member"
            # Each member is padded to an even length so the next header
            # always starts on an even offset.
            if [ $((size % 2)) -ne 0 ]; then
                printf '\n'
            fi
        done
    } > "$OUTPUT_ABS"
)

echo "built $OUTPUT_ABS"
