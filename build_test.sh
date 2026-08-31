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
cleanup() {
    rm -rf "$TMP_HOME" "$GO_STUB_LOG" \
        "${FAKE_GO_STAGE:-}" "${FAKE_GO_TARBALL:-}" "${CURL_URL_LOG:-}" \
        "${INSTALL_HOME:-}" "${INSTALL_HOME2:-}" 2>/dev/null || true
}
trap cleanup EXIT

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
echo "=== 测试 5: --help 文档包含 --go-download-url 说明 ==="
assert_contains "--help 列出 --go-download-url 选项" "$help_out" "--go-download-url <url>"
assert_contains "--help 列出 GO_DOWNLOAD_URL 环境变量" "$help_out" "GO_DOWNLOAD_URL"

echo ""
echo "=== 测试 6: install_go 指定 GO_DOWNLOAD_URL 时不探测版本 ==="
# 准备伪造的 go 安装包 (tar.gz 内含 go/bin/go 桩)
FAKE_GO_STAGE="$(mktemp -d)"
mkdir -p "$FAKE_GO_STAGE/go/bin"
cat > "$FAKE_GO_STAGE/go/bin/go" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "version" ]; then
    echo "go version go0.0.0 (stub)"
fi
exit 0
EOF
chmod +x "$FAKE_GO_STAGE/go/bin/go"
FAKE_GO_TARBALL="$(mktemp)"
tar -C "$FAKE_GO_STAGE" -czf "$FAKE_GO_TARBALL" go

# source build.sh 以便直接调用 install_go 函数
source "$BUILD_SH"
set +e # build.sh 顶部启用了 set -e，测试需要容忍断言失败

CURL_URL_LOG="$(mktemp)"
# 用 shell 函数覆盖 curl: 记录请求的 URL，下载请求返回伪造安装包
curl() {
    local url="" out="" prev="" a
    for a in "$@"; do
        case "$a" in
            http*) url="$a" ;;
        esac
        if [ "$prev" = "-o" ]; then
            out="$a"
        fi
        prev="$a"
    done
    echo "$url" >> "$CURL_URL_LOG"
    if [ -n "$out" ]; then
        cp "$FAKE_GO_TARBALL" "$out"
    else
        echo "go1.99.0" # VERSION 查询的响应
    fi
    return 0
}

INSTALL_HOME="$(mktemp -d)"
: > "$CURL_URL_LOG"
(
    HOME="$INSTALL_HOME"
    GO_DOWNLOAD_URL="https://mirror.example.com/golang/go1.22.5.linux-amd64.tar.gz"
    install_go
) > /dev/null 2>&1
assert_equals "install_go (指定 URL) 退出码为 0" "$?" "0"
assert_equals "仅请求了指定的下载地址" "$(cat "$CURL_URL_LOG")" \
    "https://mirror.example.com/golang/go1.22.5.linux-amd64.tar.gz"
if [ -x "$INSTALL_HOME/.local/go/bin/go" ]; then
    pass "go 已解压安装到 \$HOME/.local/go"
else
    fail "go 已解压安装到 \$HOME/.local/go"
fi

echo ""
echo "=== 测试 7: install_go 未指定 URL 时仍走版本探测 ==="
INSTALL_HOME2="$(mktemp -d)"
: > "$CURL_URL_LOG"
(
    HOME="$INSTALL_HOME2"
    GO_DOWNLOAD_URL=""
    install_go
) > /dev/null 2>&1
assert_equals "install_go (默认) 退出码为 0" "$?" "0"
expected_urls="$(printf 'https://go.dev/VERSION?m=text\nhttps://go.dev/dl/go1.99.0.%s-%s.tar.gz' \
    "$(detect_os)" "$(detect_arch)")"
assert_equals "先探测版本再拼接下载地址" "$(cat "$CURL_URL_LOG")" "$expected_urls"

echo ""
echo "========================================"
echo "通过: $PASS, 失败: $FAIL"
echo "========================================"
[ "$FAIL" -eq 0 ]
