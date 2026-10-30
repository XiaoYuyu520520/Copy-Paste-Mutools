#!/bin/bash
set -e

MIRROR="https://pypi.tuna.tsinghua.edu.cn/simple"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "========================================="
echo "  Hardware Monitor - 一键启动"
echo "========================================="

# 检查 Python
if ! command -v python3 &>/dev/null; then
    echo "[ERROR] 未找到 python3，请先安装 Python 3.10+"
    exit 1
fi
echo "[OK] Python $(python3 --version | cut -d' ' -f2)"

# # 安装/检查核心依赖
# MISSING=""
# python3 -c "import flask" 2>/dev/null || MISSING="$MISSING flask"
# python3 -c "import psutil" 2>/dev/null || MISSING="$MISSING psutil"

# if [ -n "$MISSING" ]; then
#     echo "[INFO] 安装核心依赖:$MISSING"
#     pip install $MISSING -i "$MIRROR" 2>&1 | tail -1
# fi
# echo "[OK] 核心依赖就绪"

# # jtop 可选安装 (Jetson 平台)，失败则忽略
# echo "[INFO] 检查 jtop (Jetson GPU)..."
# python3 -c "import jtop" 2>/dev/null || {
#     echo "[INFO] 尝试安装 jtop..."
#     pip install jtop -i "$MIRROR" 2>&1 | tail -1 || echo "[WARN] jtop 安装失败，已忽略（非 Jetson 平台可忽略此警告）"
# }
# echo "[OK] jtop 检查完成"

# 检查端口
PORT=${PORT:-8888}
if lsof -i :$PORT -sTCP:LISTEN 2>/dev/null | grep -q .; then
    echo "[WARN] 端口 $PORT 已被占用，尝试释放..."
    lsof -ti :$PORT | xargs kill -9 2>/dev/null || true
    sleep 1
fi

echo ""
echo "========================================="
echo "  启动服务 -> http://localhost:$PORT"
echo "========================================="

exec python3 app.py
