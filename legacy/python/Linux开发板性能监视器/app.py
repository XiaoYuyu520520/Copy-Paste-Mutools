import json
import re
import time
import subprocess
import platform
from pathlib import Path
from flask import Flask, Response, request, send_from_directory

from collector import system_collector, gpu_collector

# ==========================================
# 配置参数
# ==========================================
HOST = "0.0.0.0"
PORT = 8888
DEBUG = False

# 轮询时间间隔 (秒)
STREAM_INTERVAL_SEC = 1
PING_INTERVAL_SEC = 1

# Ping 配置
PING_TIMEOUT_SEC = 3
PING_COUNT = 1
# ==========================================

BASE = Path(__file__).resolve().parent
app = Flask(__name__, static_folder=str(BASE / "static"), static_url_path="")


def is_valid_target(target):
    """验证 target 是否为合法的 IP 或域名，防止异常参数注入"""
    # 简单的 IP 或域名正则
    pattern = re.compile(
        r'^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*'
        r'([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$'
    )
    return bool(pattern.match(target))


def get_ping_command(target):
    """根据操作系统生成对应的 ping 命令"""
    sys_name = platform.system().lower()
    if sys_name == "windows":
        # Windows: -n 次数, -w 超时(毫秒)
        return ["ping", "-n", str(PING_COUNT), "-w", str(PING_TIMEOUT_SEC * 1000), target]
    else:
        # Linux/macOS: -c 次数, -W 超时(秒)
        return ["ping", "-c", str(PING_COUNT), "-W", str(PING_TIMEOUT_SEC), target]


@app.route("/")
def index():
    return send_from_directory(str(BASE / "static"), "index.html")


@app.route("/api/status")
def api_status():
    data = _collect_all()
    return Response(
        json.dumps(data, ensure_ascii=False),
        mimetype="application/json",
        headers={"Access-Control-Allow-Origin": "*"}
    )


@app.route("/api/stream")
def api_stream():
    def generate():
        try:
            while True:
                data = _collect_all()
                yield f"data: {json.dumps(data, ensure_ascii=False)}\n\n"
                time.sleep(STREAM_INTERVAL_SEC)
        except GeneratorExit:
            # 客户端断开连接时优雅退出
            pass

    return Response(
        generate(),
        mimetype="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "Access-Control-Allow-Origin": "*"
        }
    )


@app.route("/api/ping/stream")
def api_ping_stream():
    target = request.args.get("target", "").strip()
    
    if not target or not is_valid_target(target):
        return Response("data: {\"error\":\"invalid or no target\"}\n\n", mimetype="text/event-stream")

    ping_cmd = get_ping_command(target)

    def generate():
        try:
            while True:
                try:
                    result = subprocess.run(
                        ping_cmd,
                        capture_output=True, 
                        text=True, 
                        timeout=PING_TIMEOUT_SEC + 1 # 略大于 ping 自身的超时
                    )
                    
                    if result.returncode == 0:
                        # 兼容 Windows 和 Linux 的时间解析
                        m = re.search(r'(?:time=|时间[=<])([0-9.]+)\s*ms', result.stdout, re.IGNORECASE)
                        latency = round(float(m.group(1)), 1) if m else None
                        payload = {"alive": True, "latency_ms": latency}
                    else:
                        payload = {"alive": False, "latency_ms": None}
                except subprocess.TimeoutExpired:
                    payload = {"alive": False, "latency_ms": None}
                except Exception:
                    payload = {"alive": False, "latency_ms": None}

                yield f"data: {json.dumps(payload, ensure_ascii=False)}\n\n"
                time.sleep(PING_INTERVAL_SEC)
        except GeneratorExit:
            pass

    return Response(
        generate(),
        mimetype="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "Access-Control-Allow-Origin": "*"
        }
    )


def _collect_all():
    data = system_collector.collect()
    try:
        data["gpu"] = gpu_collector.collect()
    except Exception:
        data["gpu"] = None
    return data


if __name__ == "__main__":
    # 注意：如果存在多并发 SSE 需求，建议使用 gunicorn + gevent 运行此应用
    app.run(host=HOST, port=PORT, debug=DEBUG)