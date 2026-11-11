#!/usr/bin/env bash
# Runs e2e/playwright/fake-confluence in the background for hands-on work, with the same port,
# site URL and Forge secret the Playwright suite uses. The suite starts its own fake on that port,
# so stop this one before `make e2e`.
#   .cursor/fake-confluence.sh start|stop|restart|status
set -euo pipefail

cd "$(dirname "$0")/../e2e/playwright"

# Same resolution as support/constants.ts and build/custom.mk: shell, then .env, then 8092.
PORT="${FAKE_CONFLUENCE_PORT:-$(sed -n 's/^FAKE_CONFLUENCE_PORT=//p' .env 2>/dev/null || true)}"
export FAKE_CONFLUENCE_PORT="${PORT:=8092}"
PID_FILE=/tmp/fake-confluence.pid
LOG_FILE=/tmp/fake-confluence.log

log() { echo "[fake-confluence] $*"; }
running() { curl -fsS "http://127.0.0.1:$PORT/__control/forge-events" >/dev/null 2>&1; }

start() {
    if running; then
        log "Already running on :$PORT."
        return 0
    fi
    setsid nohup tsx --eval "
        import {startFakeConfluence} from './fake-confluence/server';
        import {fakeCloudSiteURL, fakeConfluencePort, forgeSharedSecret} from './support/constants';
        startFakeConfluence({port: fakeConfluencePort, siteURL: fakeCloudSiteURL, forgeSecret: forgeSharedSecret});
    " >"$LOG_FILE" 2>&1 &
    echo $! >"$PID_FILE"
    for _ in $(seq 1 30); do
        if running; then
            log "Running on :$PORT (logs: $LOG_FILE)."
            return 0
        fi
        sleep 1
    done
    log "Did not start within 30 seconds:" >&2
    cat "$LOG_FILE" >&2
    exit 1
}

stop() {
    if [[ -f "$PID_FILE" ]]; then
        kill -- "-$(cat "$PID_FILE")" 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
    for _ in $(seq 1 10); do
        running || { log "Stopped."; return 0; }
        sleep 1
    done
    log "Something else is still serving :$PORT." >&2
    exit 1
}

case "${1:-start}" in
    start) start ;;
    stop) stop ;;
    restart) stop && start ;;
    status) running && log "Running on :$PORT." || { log "Not running."; exit 1; } ;;
    *) echo "usage: $0 start|stop|restart|status" >&2; exit 2 ;;
esac
