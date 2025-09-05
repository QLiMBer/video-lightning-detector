#!/usr/bin/env bash
set -euo pipefail

# Bench harness wrapper. Builds the binary if missing, then runs end-to-end
# benchmarks using VLD_CLI_ARGS. Optional: PROFILE=1 to emit cpu.prof/mem.prof.

if [[ -f "./env.sh" ]]; then
  # Use project-local Go and ffmpeg if configured
  # shellcheck disable=SC1091
  source ./env.sh
fi

COUNT=${COUNT:-5}
PROFILE=${PROFILE:-0}

if [[ -z "${VLD_CLI_ARGS:-}" ]]; then
  export VLD_CLI_ARGS='-i resources/samples/sample_yes.mp4 -o runs/bench -a -s 0.4 -f'
fi

echo "VLD_CLI_ARGS=$VLD_CLI_ARGS"

BIN=./bin/video-lightning-detector
if [[ ! -x "$BIN" ]]; then
  echo "Building binary to $BIN"
  go build -v -o "$BIN" .
fi

set -x
if [[ "$PROFILE" == "1" ]]; then
  go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem -count "$COUNT" -cpuprofile cpu.prof -memprofile mem.prof
else
  go test -v -run ^$ -bench BenchmarkVideoLightningDetectorFromEnvArgs -benchmem -count "$COUNT"
fi

