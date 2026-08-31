#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
DIST_DIR="$PROJECT_ROOT/dist"
VERSION_FILE="$PROJECT_ROOT/VERSION"
BINARY_NAME="new-api"

SKIP_FRONTEND="${SKIP_FRONTEND:-false}"
SKIP_BACKEND="${SKIP_BACKEND:-false}"
# Go 模块代理，可通过 --goproxy 参数或环境变量 GOPROXY 设置
# 国内编译机可设置为: https://goproxy.cn,direct
GOPROXY="${GOPROXY:-}"
# vite 构建大项目时 Node 默认堆内存不足，可调大上限避免 OOM
NODE_MAX_OLD_SPACE_SIZE="${NODE_MAX_OLD_SPACE_SIZE:-4096}"
export NODE_OPTIONS="--max-old-space-size=${NODE_MAX_OLD_SPACE_SIZE} ${NODE_OPTIONS:-}"

# go 可能未安装，此时回退到 uname 检测
detect_os() {
    uname -s | tr '[:upper:]' '[:lower:]'
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              uname -m ;;
    esac
}

GOOS="${GOOS:-$(command -v go >/dev/null 2>&1 && go env GOOS || detect_os)}"
GOARCH="${GOARCH:-$(command -v go >/dev/null 2>&1 && go env GOARCH || detect_arch)}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*" >&2; }

install_bun() {
    info "未检测到 bun，尝试自动安装 ..."
    if ! command -v curl >/dev/null 2>&1; then
        err "缺少 curl，无法自动安装 bun，请手动安装: https://bun.sh"
        exit 1
    fi
    curl -fsSL https://bun.sh/install | bash
    # 将 bun 默认安装目录加入当前 PATH
    export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
    export PATH="$BUN_INSTALL/bin:$PATH"
    if command -v bun >/dev/null 2>&1; then
        ok "bun 安装成功: $(bun --version)"
    else
        err "bun 安装失败，请手动安装: https://bun.sh"
        exit 1
    fi
}

install_go() {
    info "未检测到 go，尝试自动安装 ..."
    if ! command -v curl >/dev/null 2>&1; then
        err "缺少 curl，无法自动安装 go，请手动安装: https://go.dev/dl/"
        exit 1
    fi

    local version
    version="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1 || true)"
    if [ -z "$version" ]; then
        err "无法获取 Go 最新版本，请手动安装: https://go.dev/dl/"
        exit 1
    fi

    local os arch install_dir="$HOME/.local"
    os="$(detect_os)"
    arch="$(detect_arch)"

    local download_url="https://go.dev/dl/${version}.${os}-${arch}.tar.gz"
    info "下载 ${version} (${os}/${arch}) ..."
    info "下载地址: $download_url"
    local tmp
    tmp="$(mktemp -d)"
    if ! curl -fsSL "$download_url" -o "$tmp/go.tar.gz"; then
        rm -rf "$tmp"
        err "Go 下载失败: $download_url"
        err "请手动安装: https://go.dev/dl/"
        exit 1
    fi

    rm -rf "$install_dir/go"
    mkdir -p "$install_dir"
    tar -C "$install_dir" -xzf "$tmp/go.tar.gz"
    rm -rf "$tmp"

    export PATH="$install_dir/go/bin:$PATH"
    if command -v go >/dev/null 2>&1; then
        ok "go 安装成功: $(go version)"
    else
        err "go 安装失败，请手动安装: https://go.dev/dl/"
        exit 1
    fi
}

check_deps() {
    if [ "$SKIP_BACKEND" = "false" ]; then
        # 先将 go 默认安装目录加入 PATH，避免已安装但未配置 PATH 时误判
        export PATH="$HOME/.local/go/bin:/usr/local/go/bin:$PATH"
        if ! command -v go >/dev/null 2>&1; then
            install_go
        fi
    fi
    if [ "$SKIP_FRONTEND" = "false" ]; then
        # 先将 bun 默认安装目录加入 PATH，避免已安装但未配置 PATH 时误判
        export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
        export PATH="$BUN_INSTALL/bin:$PATH"
        if ! command -v bun >/dev/null 2>&1; then
            install_bun
        fi
    fi
}

get_version() {
    if [ -f "$VERSION_FILE" ] && [ -s "$VERSION_FILE" ]; then
        cat "$VERSION_FILE"
    else
        echo "dev"
    fi
}

build_frontend_default() {
    info "编译默认前端 (web/default) ..."
    local frontend_dir="$PROJECT_ROOT/web/default"
    if [ ! -d "$frontend_dir" ]; then
        err "前端目录不存在: $frontend_dir"
        exit 1
    fi
    cd "$frontend_dir"
    bun install
    DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION="$(get_version)" bun run build
    ok "默认前端编译完成"
}

build_frontend_classic() {
    info "编译经典前端 (web/classic) ..."
    local frontend_dir="$PROJECT_ROOT/web/classic"
    if [ ! -d "$frontend_dir" ]; then
        warn "经典前端目录不存在: $frontend_dir，跳过"
        return
    fi
    cd "$frontend_dir"
    bun install
    VITE_REACT_APP_VERSION="$(get_version)" bun run build
    ok "经典前端编译完成"
}

build_frontend() {
    if [ "$SKIP_FRONTEND" = "true" ]; then
        warn "跳过前端编译 (SKIP_FRONTEND=true)"
        if [ ! -d "$PROJECT_ROOT/web/default/dist" ] || [ ! -d "$PROJECT_ROOT/web/classic/dist" ]; then
            err "前端 dist 目录不存在，Go 编译需要 go:embed 的前端产物"
            err "请先编译前端或创建占位文件"
            exit 1
        fi
        return
    fi

    build_frontend_default
    build_frontend_classic
}

build_backend() {
    if [ "$SKIP_BACKEND" = "true" ]; then
        warn "跳过后端编译 (SKIP_BACKEND=true)"
        return
    fi

    info "编译后端 (Go) ..."
    cd "$PROJECT_ROOT"

    local version
    version="$(get_version)"
    local ldflags="-s -w -X 'github.com/QuantumNous/new-api/common.Version=${version}'"

    info "目标平台: GOOS=$GOOS GOARCH=$GOARCH"
    info "版本: $version"

    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -ldflags "$ldflags" -o "$DIST_DIR/$BINARY_NAME" .

    ok "后端编译完成: $DIST_DIR/$BINARY_NAME"
}

copy_extra_files() {
    info "复制附加文件 ..."
    cd "$PROJECT_ROOT"
    for f in LICENSE NOTICE THIRD-PARTY-LICENSES.md; do
        if [ -f "$f" ]; then
            cp "$f" "$DIST_DIR/"
        fi
    done
    ok "附加文件复制完成"
}

usage() {
    echo "用法: $0 [选项]"
    echo ""
    echo "编译 new-api 项目，产物输出到 dist/ 目录"
    echo ""
    echo "选项:"
    echo "  --skip-frontend    跳过前端编译 (需要已有前端 dist 产物)"
    echo "  --skip-backend     跳过后端编译"
    echo "  --goos <os>        目标操作系统 (默认: 当前系统)"
    echo "  --goarch <arch>    目标架构 (默认: 当前架构)"
    echo "  --goproxy <proxy>  设置 Go 模块代理 (例如: https://goproxy.cn,direct)"
    echo "  -h, --help         显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  SKIP_FRONTEND      设为 true 跳过前端编译"
    echo "  SKIP_BACKEND       设为 true 跳过后端编译"
    echo "  GOOS               目标操作系统"
    echo "  GOARCH             目标架构"
    echo "  GOPROXY            Go 模块代理 (同 --goproxy)"
    echo ""
    echo "示例:"
    echo "  $0                              # 完整编译"
    echo "  $0 --skip-frontend              # 仅编译后端 (前端已编译)"
    echo "  $0 --goos linux --goarch arm64  # 交叉编译 Linux ARM64"
    echo "  $0 --goproxy https://goproxy.cn,direct  # 使用国内模块代理编译"
    echo "  SKIP_FRONTEND=true $0           # 通过环境变量跳过前端"
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-frontend) SKIP_FRONTEND="true"; shift ;;
            --skip-backend)  SKIP_BACKEND="true";  shift ;;
            --goos)          GOOS="$2"; shift 2 ;;
            --goarch)        GOARCH="$2"; shift 2 ;;
            --goproxy)       GOPROXY="$2"; shift 2 ;;
            -h|--help)       usage; exit 0 ;;
            *)               err "未知参数: $1"; usage; exit 1 ;;
        esac
    done
}

main() {
    parse_args "$@"

    # 显式指定了模块代理时导出，供 go 命令使用
    if [ -n "$GOPROXY" ]; then
        export GOPROXY
        info "使用 GOPROXY: $GOPROXY"
    fi

    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  new-api 编译脚本${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""

    check_deps

    mkdir -p "$DIST_DIR"

    build_frontend
    build_backend
    copy_extra_files

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  编译完成!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "产物目录: $DIST_DIR/"
    if [ "$SKIP_BACKEND" = "false" ]; then
        echo "可执行文件: $DIST_DIR/$BINARY_NAME"
        echo ""
        echo "运行方式:"
        echo "  cd $DIST_DIR && ./new-api"
    fi
}

main "$@"
