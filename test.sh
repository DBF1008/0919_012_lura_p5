#!/usr/bin/env bash
#
# test.sh - unit test entrypoint for the concurrency fixes in merging/register
#
# Usage:
#   ./test.sh            # default: full unit test suite with -race
#   ./test.sh quick      # only the packages touched by the fix
#                        # (register + proxy, network-free tests)
#   ./test.sh race       # only the new concurrency regression tests, -race
#   ./test.sh vet        # go vet ./...
#   ./test.sh build      # go build ./...
#
# The quick/race modes skip tests that bind local TCP ports (httptest), so they
# work in network-restricted sandboxes. The default mode runs everything and
# needs loopback network access.
set -euo pipefail

cd "$(dirname "$0")"

export GOFLAGS="${GOFLAGS:--mod=readonly}"

mode="${1:-all}"

# network-dependent test name fragments (httptest.NewServer needs a free port)
SKIP_NET='HTTP|TLS|Graphql|Server|Plugin|LoadBalancing|Subscriber|Async|Backoff'

case "$mode" in
  quick)
    echo "==> quick: register + proxy, -race, network-free tests only"
    go test -race -count=1 -skip "$SKIP_NET" ./register/ ./proxy/
    ;;
  race)
    echo "==> race: concurrency regression tests with -race"
    go test -race -count=1 -run 'TestUntyped_concurrentAccess|TestNamespaced_concurrentRegister' ./register/
    go test -race -count=1 -run 'TestRace_' ./proxy/
    ;;
  vet)
    echo "==> go vet ./..."
    go vet ./...
    ;;
  build)
    echo "==> go build ./..."
    go build ./...
    ;;
  all)
    echo "==> go vet ./..."
    go vet ./...
    echo "==> go build ./..."
    go build ./...
    echo "==> go test -race ./..."
    go test -race -count=1 ./...
    echo "==> concurrency regression tests"
    go test -race -count=1 -run 'TestUntyped_concurrentAccess|TestNamespaced_concurrentRegister' ./register/
    go test -race -count=1 -run 'TestRace_' ./proxy/
    echo "==> all checks passed"
    ;;
  *)
    echo "unknown mode: $mode" >&2
    echo "usage: $0 [all|quick|race|vet|build]" >&2
    exit 2
    ;;
esac
