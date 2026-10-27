#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "$0")"
mode="${1:-preview}"
if [[ "$mode" == "preview" ]]; then
  exec python3 -m http.server 5173 --bind 127.0.0.1 --directory frontend
fi
space_go="$(command -v go || true)"
if [[ -z "$space_go" && -x /home/enowx/.local/opt/go/bin/go ]]; then
  space_go=/home/enowx/.local/opt/go/bin/go
fi
if [[ -z "$space_go" ]]; then
  printf '%s\n' 'Go 1.27.3+ is needed for the MyGo desktop launcher.' >&2
  exit 1
fi
if [[ "$mode" == "desktop" ]]; then
	if [[ "${2:-}" == "refined" ]]; then
		exec "$space_go" run . -refined
	fi
  exec "$space_go" run . -variant "${2:-A}"
fi
if [[ "$mode" == "build-windows" ]]; then
  mkdir -p artifacts
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$space_go" build -o artifacts/space-ui-windows-amd64.exe .
  exit
fi
printf '%s\n' 'Usage: ./run.sh preview | desktop [refined|A-E] | build-windows' >&2
exit 1
