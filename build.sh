#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
DIST_DIR="$PROJECT_ROOT/dist"
VERSION_FILE="$PROJECT_ROOT/VERSION"
BINARY_NAME="new-api"

SKIP_FRONTEND="${SKIP_FRONTEND:-false}"
SKIP_BACKEND="${SKIP_BACKEND:-false}"
GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

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

check_deps() {
    local missing=()
    if [ "$SKIP_BACKEND" = "false" ]; then
        command -v go >/dev/null 2>&1 || missing+=("go")
    fi
    if [ "$SKIP_FRONTEND" = "false" ]; then
        # 先将 bun 默认安装目录加入 PATH，避免已安装但未配置 PATH 时误判
        export BUN_INSTALL="${BUN_INSTALL:-$HOME/.bun}"
        export PATH="$BUN_INSTALL/bin:$PATH"
        if ! command -v bun >/dev/null 2>&1; then
            install_bun
        fi
    fi
    if [ ${#missing[@]} -ne 0 ]; then
        err "缺少必要依赖: ${missing[*]}"
        err "请安装后再运行"
        exit 1
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
    echo "  -h, --help         显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  SKIP_FRONTEND      设为 true 跳过前端编译"
    echo "  SKIP_BACKEND       设为 true 跳过后端编译"
    echo "  GOOS               目标操作系统"
    echo "  GOARCH             目标架构"
    echo ""
    echo "示例:"
    echo "  $0                              # 完整编译"
    echo "  $0 --skip-frontend              # 仅编译后端 (前端已编译)"
    echo "  $0 --goos linux --goarch arm64  # 交叉编译 Linux ARM64"
    echo "  SKIP_FRONTEND=true $0           # 通过环境变量跳过前端"
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-frontend) SKIP_FRONTEND="true"; shift ;;
            --skip-backend)  SKIP_BACKEND="true";  shift ;;
            --goos)          GOOS="$2"; shift 2 ;;
            --goarch)        GOARCH="$2"; shift 2 ;;
            -h|--help)       usage; exit 0 ;;
            *)               err "未知参数: $1"; usage; exit 1 ;;
        esac
    done
}

main() {
    parse_args "$@"

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
