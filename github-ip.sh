#!/bin/sh
# 说明：用 /bin/sh 作为入口以兼容 Termux（无 /usr/bin/env），检测到 bash 后自动切换。
if [ -z "${BASH_VERSION:-}" ] && command -v bash >/dev/null 2>&1; then
  exec bash "$0" "$@"
fi
# github-ip.sh — 一键探测可用的 GitHub IP；可选用本地 CONNECT 代理执行 git push
#
# 用法：
#   ./github-ip.sh                    探测候选 IP，按顺序列出可达的
#   ./github-ip.sh --best             仅输出第一个可达 IP（供脚本调用）
#   ./github-ip.sh --ip 1.2.3.4 ...   追加候选 IP（优先探测，可重复）
#   ./github-ip.sh --push [git 参数]  自动选可达 IP，起本地代理后执行 git
#                                     （默认 `git push origin main`）
#
# 说明：
#   - 探测使用 curl --resolve，无需 root、不改 hosts
#   - --push 需要本机有 Go：脚本会在临时目录生成一个 CONNECT 代理并
#     `go run` 启动，github.com 固定指向选中 IP；TLS 仍按原域名校验
#   - 环境变量：GH_IP 强制指定 IP；GH_PROXY_PORT 代理端口（默认 18081）
set -uo pipefail

CANDIDATES="20.27.177.113 140.82.112.3 140.82.113.3 140.82.114.3 140.82.116.3
20.205.243.166 20.26.156.215 20.201.28.151 20.248.137.48 4.237.22.38"
TIMEOUT=5
ROUNDS=6
PORT="${GH_PROXY_PORT:-18081}"

usage() {
  cat <<'USAGE'
github-ip.sh — 一键探测可用的 GitHub IP；可选用本地 CONNECT 代理执行 git push

用法：
  ./github-ip.sh                    探测候选 IP，按顺序列出可达的
  ./github-ip.sh --best             仅输出第一个可达 IP（供脚本调用）
  ./github-ip.sh --ip 1.2.3.4 ...   追加候选 IP（优先探测，可重复）
  ./github-ip.sh --push [git 参数]  自动选可达 IP，起本地代理后执行 git
                                    （默认 `git push origin main`）

说明：
  - 探测使用 curl --resolve，无需 root、不改 hosts
  - --push 需要本机有 Go：脚本在临时目录生成 CONNECT 代理并 go run 启动，
    github.com 固定指向选中 IP；TLS 仍按原域名校验
  - 环境变量：GH_IP 强制指定 IP；GH_PROXY_PORT 代理端口（默认 18081）
USAGE
}

MODE=probe
EXTRA_IPS=()
GIT_ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --best) MODE=best; shift ;;
    --push) MODE=push; shift; GIT_ARGS=("$@"); break ;;
    --ip) [ $# -ge 2 ] || { echo "缺少 --ip 参数" >&2; exit 2; }; EXTRA_IPS+=("$2"); shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数：$1" >&2; usage >&2; exit 2 ;;
  esac
done

resolved="$(getent hosts github.com 2>/dev/null | awk '{print $1}')"
ALL="$(printf '%s\n' "${EXTRA_IPS[@]}" $CANDIDATES $resolved | awk 'NF && !seen[$0]++')"

TMP="$(mktemp -d)"
PROXY_PID=""
cleanup() {
  [ -n "$PROXY_PID" ] && kill "$PROXY_PID" 2>/dev/null
  rm -rf "$TMP"
}
trap cleanup EXIT

# probe_reachable：并行探测，结果按候选顺序写入 $TMP/ok
probe_reachable() {
  : > "$TMP/ok"
  for ip in $ALL; do
    (
      code="$(curl -s -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT" \
        --resolve "github.com:443:$ip" https://github.com/ 2>/dev/null)"
      [ "$code" = "200" ] && echo "$ip" >> "$TMP/ok"
    ) &
  done
  wait
}

print_reachable() {
  for ip in $ALL; do
    grep -qx "$ip" "$TMP/ok" 2>/dev/null && echo "$ip"
  done
}

pick_best() {
  probe_reachable
  print_reachable | head -1
}

build_proxy() {
  command -v go >/dev/null 2>&1 || {
    echo "需要 Go 才能启动本地代理（探测功能不需要）；请安装 Go 后重试" >&2
    return 1
  }
  mkdir -p "$TMP/proxy"
  cat > "$TMP/proxy/main.go" <<'GOEOF'
package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

var fixedIP = os.Getenv("GH_IP")

func main() {
	port := os.Getenv("GH_PROXY_PORT")
	if port == "" {
		port = "18081"
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("proxy 127.0.0.1:%s -> github.com:%s\n", port, fixedIP)
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go handle(c)
	}
}

func handle(c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil || req.Method != http.MethodConnect {
		return
	}
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		return
	}
	dial := host
	if host == "github.com" && fixedIP != "" {
		dial = fixedIP
	}
	up, err := net.DialTimeout("tcp", net.JoinHostPort(dial, port), 10*time.Second)
	if err != nil {
		fmt.Fprint(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer up.Close()
	fmt.Fprint(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(up, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(c, up); done <- struct{}{} }()
	<-done
}
GOEOF
  ( cd "$TMP/proxy" && go build -o "$TMP/ghproxy" main.go ) || return 1
  GH_IP="$ip" GH_PROXY_PORT="$PORT" "$TMP/ghproxy" >"$TMP/proxy.log" 2>&1 &
  PROXY_PID=$!
  # 等代理就绪（最多 15s）
  for _ in $(seq 1 50); do
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 2 \
      --proxy "http://127.0.0.1:$PORT" https://github.com/ 2>/dev/null)"
    [ "$code" = "200" ] && return 0
    sleep 0.3
  done
  echo "代理启动失败：" >&2
  tail -3 "$TMP/proxy.log" >&2
  return 1
}

case "$MODE" in
  probe)
    probe_reachable
    ips="$(print_reachable)"
    if [ -z "$ips" ]; then
      echo "未找到可达的 GitHub IP（网络可能整体受阻）" >&2
      exit 1
    fi
    echo "可达的 GitHub IP（按候选顺序）："
    echo "$ips" | sed 's/^/  /'
    ;;
  best)
    ip="$(pick_best)"
    if [ -z "$ip" ]; then
      echo "未找到可达的 GitHub IP" >&2
      exit 1
    fi
    echo "$ip"
    ;;
  push)
    [ ${#GIT_ARGS[@]} -eq 0 ] && GIT_ARGS=(push origin main)
    ip="${GH_IP:-}"
    for round in $(seq 1 "$ROUNDS"); do
      [ -n "$ip" ] || ip="$(pick_best)"
      if [ -z "$ip" ]; then
        echo "第 $round/$ROUNDS 轮：暂无可达 IP，10s 后重试…" >&2
        sleep 10
        continue
      fi
      echo "使用 IP：$ip"
      if start_proxy "$ip"; then
        echo "代理已就绪，执行：git ${GIT_ARGS[*]}"
        if HTTPS_PROXY="http://127.0.0.1:$PORT" HTTP_PROXY="http://127.0.0.1:$PORT" \
           git "${GIT_ARGS[@]}"; then
          exit 0
        fi
        echo "git 执行失败，换 IP 重试…" >&2
      fi
      kill "$PROXY_PID" 2>/dev/null
      PROXY_PID=""
      ip=""
      sleep 3
    done
    echo "多次重试后仍未成功" >&2
    exit 1
    ;;
esac
