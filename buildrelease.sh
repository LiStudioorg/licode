#!/bin/bash
set -e

# ─────────────────────────────────────────────────────────
#  通用配置
# ─────────────────────────────────────────────────────────
APP_NAME="${APP_NAME:-$(basename "${GITHUB_REPOSITORY:-myapp}")}"
VERSION="${VERSION:-${GITHUB_REF_NAME:-dev}}"
VERSION="${VERSION#v}"
DIST_DIR="dist"

# NDK 配置
NDK_VERSION="r26b"
NDK_DIR="android-ndk-${NDK_VERSION}"
NDK_URL="https://dl.google.com/android/repository/${NDK_DIR}-linux.zip"

# ─────────────────────────────────────────────────────────
#  版本注入开关
#  ENABLE_VERSION_INJECT=1 时，向二进制注入版本号
#  前提：main 包中必须有 `var version = "dev"` 这样的字符串变量
#  如果你改成其它包，请改 VERSION_SYMBOL 前缀
# ─────────────────────────────────────────────────────────
ENABLE_VERSION_INJECT="${ENABLE_VERSION_INJECT:-1}"
VERSION_SYMBOL="${VERSION_SYMBOL:-main.version}"

if [ "$ENABLE_VERSION_INJECT" = "1" ]; then
  LDFLAGS_BASE="-s -w -X ${VERSION_SYMBOL}=${VERSION}"
else
  LDFLAGS_BASE="-s -w"
fi

# ─────────────────────────────────────────────────────────
#  工具函数
# ─────────────────────────────────────────────────────────
log()  { echo -e "\033[1;34m[INFO]\033[0m $*"; }
warn() { echo -e "\033[1;33m[WARN]\033[0m $*"; }
err()  { echo -e "\033[1;31m[ERR ]\033[0m $*" >&2; }

get_ext() {
  case "$1" in
    windows) echo ".exe" ;;
    js|wasip1) echo ".wasm" ;;
    *) echo "" ;;
  esac
}

# ─────────────────────────────────────────────────────────
#  Android：下载（或复用缓存）NDK + 指定 CC
# ─────────────────────────────────────────────────────────
prepare_android() {
  local arch="$1"
  log "准备 Android NDK (${NDK_VERSION}) ..."

  if [ -d "$NDK_DIR" ]; then
    log "✅ 复用缓存 NDK 目录: $NDK_DIR"
  else
    log "下载 NDK：$NDK_URL"
    curl -sSL -o "${NDK_DIR}-linux.zip" "$NDK_URL"
    unzip -q "${NDK_DIR}-linux.zip"
    rm -f "${NDK_DIR}-linux.zip"
  fi

  local ndk_bin="${PWD}/${NDK_DIR}/toolchains/llvm/prebuilt/linux-x86_64/bin"
  case "$arch" in
    arm64) export CC="$ndk_bin/aarch64-linux-android24-clang" ;;
    arm)   export CC="$ndk_bin/armv7a-linux-androideabi24-clang" ;;
    amd64) export CC="$ndk_bin/x86_64-linux-android24-clang" ;;
    386)   export CC="$ndk_bin/i686-linux-android24-clang" ;;
    *) err "不支持的 Android 架构: $arch"; return 1 ;;
  esac

  export CGO_ENABLED=1
  log "Android CC = $CC"
}

# ─────────────────────────────────────────────────────────
#  iOS：使用 Xcode clang，产物为 c-archive
# ─────────────────────────────────────────────────────────
prepare_ios() {
  local arch="$1"
  log "准备 iOS 工具链 (arch=$arch) ..."

  local sdk
  if [ "$arch" = "arm64" ]; then
    sdk="iphoneos"
  else
    sdk="iphonesimulator"
  fi

  export CC="$(xcrun -find clang) -arch $arch -isysroot $(xcrun -sdk $sdk --show-sdk-path)"
  export CGO_ENABLED=1
  log "iOS CC = $CC"
}

# ─────────────────────────────────────────────────────────
#  核心：编译单个平台
# ─────────────────────────────────────────────────────────
build_one() {
  local goos="$1"
  local goarch="$2"
  local ext
  ext="$(get_ext "$goos")"

  log "===== 开始编译 ${goos}/${goarch} ====="

  export GOOS="$goos"
  export GOARCH="$goarch"
  export CGO_ENABLED=0

  local buildmode=""
  local ldflags="$LDFLAGS_BASE"

  case "$goos" in
    android)
      prepare_android "$goarch" || return 1
      ;;
    ios)
      prepare_ios "$goarch" || return 1
      buildmode="-buildmode=c-archive"
      ext=".a"
      ;;
    *)
      :
      ;;
  esac

  mkdir -p "$DIST_DIR"
  local out="${DIST_DIR}/${APP_NAME}_${VERSION}_${goos}_${goarch}${ext}"

  log "go build -trimpath ${buildmode} -ldflags \"$ldflags\" -o \"$out\" ."
  go build -trimpath $buildmode -ldflags "$ldflags" -o "$out" .

  package_artifact "$goos" "$goarch" "$out"
  log "✅ 完成 ${goos}/${goarch}"
}

# ─────────────────────────────────────────────────────────
#  打包：tar.gz / zip
# ─────────────────────────────────────────────────────────
package_artifact() {
  local goos="$1" goarch="$2" src="$3"
  local base="${APP_NAME}_${VERSION}_${goos}_${goarch}"
  local fname
  fname="$(basename "$src")"

  cd "$DIST_DIR"
  if [ "$goos" = "windows" ]; then
    zip -q "${base}.zip" "$fname"
  else
    tar -czf "${base}.tar.gz" "$fname"
  fi
  rm -f "$fname"
  cd - >/dev/null
}

# ─────────────────────────────────────────────────────────
#  主入口
# ─────────────────────────────────────────────────────────
main() {
  local target="$1"

  if [ -z "$target" ]; then
    err "用法: $0 <goos>/<goarch>"
    err "例如: $0 linux/amd64"
    exit 1
  fi

  local goos="${target%/*}"
  local goarch="${target#*/}"

  build_one "$goos" "$goarch"
}

main "$@"
