# config.py

# 日志文件路径
LOG_FILE = "usb_log.md"

# 时间格式
TIME_FORMAT = "%Y-%m-%d %H:%M:%S"

# Markdown 表格头部配置
MD_SCAN_HEADER = "| 扫描时间 | 厂商/品牌 | 产品名称 | 设备路径 (Dev Node) | 系统路径 (Sys Path) |\n| --- | --- | --- | --- | --- |"
MD_MONITOR_HEADER = "| 动作 | 时间 | 厂商/品牌 | 产品名称 | 设备路径 (Dev Node) | 系统路径 (Sys Path) |\n| --- | --- | --- | --- | --- | --- |"

# 监听模式的轮询超时时间（秒）
POLL_TIMEOUT = 1.0