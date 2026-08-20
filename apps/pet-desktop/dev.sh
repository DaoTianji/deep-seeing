#!/usr/bin/env bash
# Start Tauri pet shell. Room must already listen on 127.0.0.1:3319.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="$(cd "$(dirname "$0")" && pwd)"

if [[ -d "$ROOT/.tools/cargo/bin" ]]; then
  export RUSTUP_HOME="${RUSTUP_HOME:-$ROOT/.tools/rustup}"
  export CARGO_HOME="${CARGO_HOME:-$ROOT/.tools/cargo}"
  export PATH="$CARGO_HOME/bin:$PATH"
fi

# Make sure localhost bypasses any HTTP proxy, otherwise `tauri dev` gets stuck
# waiting for the frontend dev server (its health check to 127.0.0.1:3319
# would be routed through the proxy).
LOCAL_NO_PROXY="127.0.0.1,::1,localhost"
if [[ -n "${no_proxy:-}" ]]; then
  export no_proxy="$no_proxy,$LOCAL_NO_PROXY"
else
  export no_proxy="$LOCAL_NO_PROXY"
fi
export NO_PROXY="$no_proxy"

cd "$APP"
if [[ ! -d node_modules ]]; then
  npm install
fi
exec npm run dev
