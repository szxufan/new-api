#!/usr/bin/env bash
# build.sh 的测试脚本，重点验证 --goproxy 参数与 GOPROXY 环境变量传递。
#
# 原理:
#   构造一个临时 HOME 目录，在其中放置伪造的 go 命令。
#   build.sh 的 check_deps 会将 $HOME/.local/go/bin 置于 PATH 最前，
#   因此伪造的 go 会被优先使用。伪造的 go 在收到 build 子命令时，
#   将当时环境变量 GOPROXY 的值写入日志文件，测试据此断言
#   build.sh 是否正确导出了 GOPROXY。
#
# 副作用: 会以跳过前端/桩 go 的方式运行 build.sh，
#   copy_extra_files 会把 LICENSE 等文件复制到 dist/ (幂等操作)。
#
# 用法: ./build_test.sh
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_SH="$SCRIPT_DIR/build.sh"

PASS=0
FAIL=0

pass() { echo "[PASS] $1"; PASS=$((PASS + 1)); }
fail() {
    echo "[FAIL] $1"
    if [ $# -ge 3 ]; then
        echo "       期望: $2"
        echo "       实际: $3"
    fi
    FAIL=$((FAIL + 1))
}

assert_contains() { # desc haystack needle
    if [[ "$2" == *"$3"* ]]; then pass "$1"; else fail "$1" "包含 '$3'" "$2"; fi
}

assert_not_contains() { # desc haystack needle
    if [[ "$2" != *"$3"* ]]; then pass "$1"; else fail "$1" "不包含 '$3'" "$2"; fi
}

assert_equals() { # desc actual expected
    if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "$3" "$2"; fi
}

TMP_HOME="$(mktemp -d)"
GO_STUB_DIR="$TMP_HOME/.local/go/bin"
GO_STUB_LOG="$(mktemp)"
trap 'rm -rf "$TMP_HOME" "$GO_STUB_LOG"' EXIT

mkdir -p "$GO_STUB_DIR"
cat > "$GO_STUB_DIR/go" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    env)
        case "${2:-}" in
            GOOS) echo "linux" ;;
            GOARCH) echo "amd64" ;;
            *) echo "" ;;
        esac
        ;;
    build)
        echo "GOPROXY=${GOPROXY:-}" >> "${GO_STUB_LOG:?}"
        ;;
    version)
        echo "go version go0.0.0 (stub)"
        ;;
esac
exit 0
EOF
chmod +x "$GO_STUB_DIR/go"

# 以桩 go 运行 build.sh 的公共环境设置
# $1: 日志文件; 其余参数原样传给 build.sh
run_build() {
    local log_file="$1"
    shift
    : > "$log_file"
    env -u GOPROXY HOME="$TMP_HOME" PATH="$GO_STUB_DIR:$PATH" \
        GO_STUB_LOG="$log_file" \
        "$BUILD_SH" "$@"
}

echo "=== 测试 1: --help 文档包含 --goproxy 说明 ==="
help_out="$("$BUILD_SH" --help)"
assert_contains "--help 列出 --goproxy 选项" "$help_out" "--goproxy <proxy>"
assert_contains "--help 列出 GOPROXY 环境变量" "$help_out" "GOPROXY"
assert_contains "--help 示例包含 goproxy.cn" "$help_out" "https://goproxy.cn,direct"

echo ""
echo "=== 测试 2: --goproxy 参数导出到 go build ==="
out="$(run_build "$GO_STUB_LOG" --skip-frontend --goproxy "https://goproxy.cn,direct" 2>&1)"
rc=$?
assert_equals "build.sh 退出码为 0" "$rc" "0"
assert_contains "输出提示使用的 GOPROXY" "$out" "使用 GOPROXY: https://goproxy.cn,direct"
assert_equals "go build 收到 GOPROXY" "$(cat "$GO_STUB_LOG")" "GOPROXY=https://goproxy.cn,direct"

echo ""
echo "=== 测试 3: 环境变量 GOPROXY 透传到 go build ==="
: > "$GO_STUB_LOG"
out="$(env HOME="$TMP_HOME" GOPROXY="https://goproxy.io,direct" \
    PATH="$GO_STUB_DIR:$PATH" GO_STUB_LOG="$GO_STUB_LOG" \
    "$BUILD_SH" --skip-frontend 2>&1)"
rc=$?
assert_equals "build.sh 退出码为 0" "$rc" "0"
assert_equals "go build 收到环境变量 GOPROXY" "$(cat "$GO_STUB_LOG")" "GOPROXY=https://goproxy.io,direct"

echo ""
echo "=== 测试 4: 未指定时不强制设置 GOPROXY ==="
out="$(run_build "$GO_STUB_LOG" --skip-frontend 2>&1)"
rc=$?
assert_equals "build.sh 退出码为 0" "$rc" "0"
assert_equals "go build 未收到脚本设置的 GOPROXY" "$(cat "$GO_STUB_LOG")" "GOPROXY="
assert_not_contains "未指定时不打印 GOPROXY 提示" "$out" "使用 GOPROXY"

echo ""
echo "========================================"
echo "通过: $PASS, 失败: $FAIL"
echo "========================================"
[ "$FAIL" -eq 0 ]
