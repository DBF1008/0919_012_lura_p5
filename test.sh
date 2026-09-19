#!/usr/bin/env bash
#
# test.sh - 手动运行全部单元测试（含并发安全回归测试）
#
# 用法:
#   ./test.sh            # 运行全部单元测试 + race 检测
#   ./test.sh unit       # 仅运行单元测试（不含 integration）
#   ./test.sh race       # 仅运行并发安全回归测试（重复 10 次）
#   ./test.sh integration # 仅运行集成测试（需先构建插件）
#
set -euo pipefail
cd "$(dirname "$0")"

run_unit() {
    echo "==> [1/3] go vet"
    go vet ./...

    echo "==> [2/3] 单元测试 (go test -cover -race ./...)"
    go test -cover -race -count=1 ./...

    echo "==> [3/3] 并发安全回归测试 (重复 10 次)"
    go test -race -count=10 -run 'Concurrent' -v ./proxy/ ./register/
}

run_race() {
    echo "==> 并发安全回归测试 (重复 10 次)"
    go test -race -count=10 -v \
        -run 'TestParallelMerge_Concurrent|TestSequentialMerge_Concurrent|TestCombinerRegister_Concurrent|TestUntyped_Concurrent|TestNamespaced_Concurrent' \
        ./proxy/ ./register/
}

run_integration() {
    echo "==> 构建测试插件"
    go build -buildmode=plugin -o ./transport/http/client/plugin/tests/lura-client-example.so ./transport/http/client/plugin/tests
    go build -buildmode=plugin -o ./transport/http/server/plugin/tests/lura-server-example.so ./transport/http/server/plugin/tests
    go build -buildmode=plugin -o ./proxy/plugin/tests/lura-request-modifier-example.so ./proxy/plugin/tests/logger
    go build -buildmode=plugin -o ./proxy/plugin/tests/lura-error-example.so ./proxy/plugin/tests/error

    echo "==> 集成测试 (tags: integration)"
    go test -tags integration ./test/...
    go test -tags integration ./transport/...
    go test -tags integration ./proxy/...
}

case "${1:-all}" in
    unit)        run_unit ;;
    race)        run_race ;;
    integration) run_integration ;;
    all)         run_unit; run_integration ;;
    *) echo "unknown target: $1 (expected: all|unit|race|integration)" >&2; exit 1 ;;
esac

echo "==> 全部通过 ✅"
