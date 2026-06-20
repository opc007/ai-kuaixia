#!/bin/bash
# 本地一键启动：API 服务（需先编译 backend）
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/backend"

if [ ! -f /tmp/aikuaixia-api ] || [ "backend/cmd/server/main.go" -nt /tmp/aikuaixia-api ]; then
  echo "🔨 编译后端..."
  go build -o /tmp/aikuaixia-api ./cmd/server/
fi

lsof -ti:8080 | xargs kill -9 2>/dev/null || true
sleep 1

echo "🚀 启动 API (http://localhost:8080)..."
DB_DRIVER=sqlite /tmp/aikuaixia-api &
sleep 2

if curl -sf http://localhost:8080/api/v1/platforms >/dev/null; then
  echo "✅ API 就绪"
else
  echo "❌ API 启动失败"
  exit 1
fi

echo ""
echo "📋 网页版："
echo "   GitHub Pages: https://opc007.github.io/ai-kuaixia/landing/app.html"
echo "   本地预览:     cd landing && python3 -m http.server 8081"
echo "                → http://localhost:8081/app.html"
echo ""
echo "☁️  线上 API 需部署 Render，见 DEPLOY.md"
