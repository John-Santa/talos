#!/usr/bin/env bash
# run-talos.sh — compila los CLIs de la plataforma + la consola TUI y lanza `talos`
# con todo en PATH. Corré desde cualquier lado dentro del repo/worktree de talos.
#
#   ./platform/console/run-talos.sh
#
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${TMPDIR:-/tmp}/talosbin"
mkdir -p "$BIN"

echo "▸ building wt/mo/ov/ch/talos → $BIN ..."
go -C "$ROOT/platform/worktree-orchestrator"   build -o "$BIN/wt"    ./cmd/wt
go -C "$ROOT/platform/merge-order-orchestrator" build -o "$BIN/mo"    ./cmd/mo
go -C "$ROOT/platform/overlap-guard"            build -o "$BIN/ov"    ./cmd/ov
go -C "$ROOT/platform/ci-checks"                build -o "$BIN/ch"    ./cmd/ch
go -C "$ROOT/platform/console"                  build -o "$BIN/talos" ./cmd/talos

echo "▸ launching talos — j/k mover · tab layout · ? help · q salir"
cd "$ROOT"
PATH="$BIN:$PATH" exec "$BIN/talos"
