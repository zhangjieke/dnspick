#!/bin/sh
# dnspick 安装脚本
#
# 用法：
#   curl -fsSL https://raw.githubusercontent.com/palemoky/dnspick/main/install.sh | sh
#
# 可选环境变量：
#   DNSPICK_VERSION   指定版本（默认 latest），例如 v2.0.0
#   DNSPICK_BIN_DIR   安装目录（默认优先 /usr/local/bin，无权限则回退 $HOME/.local/bin）

set -eu

REPO="palemoky/dnspick"
APP="dnspick"
VERSION="${DNSPICK_VERSION:-latest}"

# 颜色（终端为 tty 时启用）
if [ -t 1 ]; then
	RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; BLUE='\033[0;34m'; NC='\033[0m'
else
	RED=''; GREEN=''; YELLOW=''; BLUE=''; NC=''
fi
info() { printf '%b\n' "${BLUE}==>${NC} $1"; }
ok()   { printf '%b\n' "${GREEN}✓${NC} $1"; }
warn() { printf '%b\n' "${YELLOW}⚠${NC} $1"; }
die()  { printf '%b\n' "${RED}✗ $1${NC}" >&2; exit 1; }

# 1. 检测操作系统
os=$(uname -s)
case "$os" in
	Linux)  OS=linux ;;
	Darwin) OS=darwin ;;
	*) die "不支持的操作系统：$os（Windows 请下载 .zip 包，见 README）" ;;
esac

# 2. 检测 CPU 架构
arch=$(uname -m)
case "$arch" in
	x86_64|amd64)  ARCH=amd64 ;;
	arm64|aarch64) ARCH=arm64 ;;
	*) die "不支持的架构：$arch" ;;
esac

# 3. 选择下载工具
if command -v curl >/dev/null 2>&1; then
	DL='curl -fsSL -o'
elif command -v wget >/dev/null 2>&1; then
	DL='wget -qO'
else
	die "需要 curl 或 wget"
fi

# 4. 拼接下载地址
ASSET="${APP}-${OS}-${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
	URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
	URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

info "平台：${OS}/${ARCH}，版本：${VERSION}"

# 5. 下载并解压到临时目录
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
info "下载 ${ASSET} ..."
$DL "$TMP/$ASSET" "$URL" || die "下载失败：$URL"
tar -xzf "$TMP/$ASSET" -C "$TMP" || die "解压失败"

# 归档内二进制名为 dnspick-<os>-<arch>，统一重命名为 dnspick
BIN_SRC="$TMP/${APP}-${OS}-${ARCH}"
[ -f "$BIN_SRC" ] || BIN_SRC="$TMP/$APP"
[ -f "$BIN_SRC" ] || die "归档中未找到可执行文件"
chmod +x "$BIN_SRC"

# 6. 选择安装目录
if [ -n "${DNSPICK_BIN_DIR:-}" ]; then
	BIN_DIR="$DNSPICK_BIN_DIR"
elif [ -w /usr/local/bin ] 2>/dev/null || [ "$(id -u)" = "0" ]; then
	BIN_DIR=/usr/local/bin
else
	BIN_DIR="$HOME/.local/bin"
fi
mkdir -p "$BIN_DIR"

DEST="$BIN_DIR/$APP"
if mv "$BIN_SRC" "$DEST" 2>/dev/null; then :; else
	warn "需要管理员权限写入 $BIN_DIR"
	sudo mv "$BIN_SRC" "$DEST" || die "安装失败"
fi

# 7. macOS：解除 Gatekeeper 隔离标记
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
	xattr -dr com.apple.quarantine "$DEST" 2>/dev/null || true
fi

ok "已安装到 ${DEST}"

# 8. PATH 提示
case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) warn "${BIN_DIR} 不在 PATH 中，请将其加入，例如："
	   printf '    export PATH="%s:$PATH"\n' "$BIN_DIR" ;;
esac

printf '%b\n' "${GREEN}完成！${NC} 运行 ${BLUE}${APP} --help${NC} 开始使用。"
