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

rm -f "$OUTPUT_ABS"
(cd "$WORK" && ar rc "$OUTPUT_ABS" debian-binary control.tar.gz data.tar.gz)

echo "built $OUTPUT_ABS"
