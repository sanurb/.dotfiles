#!/usr/bin/env bash
set -euo pipefail
sync_script=$1
export PLUGIN_TEST_LOG="$TMPDIR/herdr-plugins.log"

herdr() {
  printf '%s\n' "$*" >> "$PLUGIN_TEST_LOG"
  case "$*" in
    'plugin install first/plugin --ref aaaa --yes') return "${INSTALL_FAILURE:-0}" ;;
    'config check') return "${CONFIG_FAILURE:-0}" ;;
    'status server') return "${SERVER_OFFLINE:-0}" ;;
  esac
}
bun() { return 0; }
export -f herdr bun

bash -eu "$sync_script" first/plugin aaaa second/plugin bbbb
cat > "$TMPDIR/expected" <<'EXPECTED'
plugin install first/plugin --ref aaaa --yes
plugin install second/plugin --ref bbbb --yes
config check
status server
server reload-config
EXPECTED
diff -u "$TMPDIR/expected" "$PLUGIN_TEST_LOG"

: > "$PLUGIN_TEST_LOG"
if INSTALL_FAILURE=1 bash -eu "$sync_script" first/plugin aaaa second/plugin bbbb; then
  echo 'plugin failure was ignored' >&2
  exit 1
fi
test "$(wc -l < "$PLUGIN_TEST_LOG")" -eq 2
! grep -q 'reload-config' "$PLUGIN_TEST_LOG"

: > "$PLUGIN_TEST_LOG"
if CONFIG_FAILURE=1 bash -eu "$sync_script" first/plugin aaaa; then
  echo 'invalid config was ignored' >&2
  exit 1
fi
! grep -q 'reload-config' "$PLUGIN_TEST_LOG"

: > "$PLUGIN_TEST_LOG"
SERVER_OFFLINE=1 bash -eu "$sync_script" first/plugin aaaa
! grep -q 'reload-config' "$PLUGIN_TEST_LOG"

echo 'Herdr plugin sync: pinned arguments, failure reporting, and reload ordering pass'
