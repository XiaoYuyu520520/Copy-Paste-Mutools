import platform
import psutil

# ==========================================
# 配置参数
# ==========================================
# 需要忽略的磁盘文件系统类型 (用于过滤内存盘、虚拟挂载等)
IGNORE_FSTYPES = {'squashfs', 'tmpfs', 'devtmpfs', 'overlay', 'shm', 'iso9660', 'autofs'}

# 需要忽略的磁盘设备前缀 (如 Ubuntu 下的 snap 虚拟 loop 设备)
IGNORE_DEVICE_PREFIXES = ('/dev/loop',)
# ==========================================


def collect_os():
    return {
        "system": platform.system(),
        "release": platform.release(),
        "version": platform.version(),
        "machine": platform.machine(),
        "node": platform.node(),
        "pretty": f"{platform.system()} {platform.release()} ({platform.machine()})"
    }


def collect_cpu():
    # 注意: 在多线程 Web 环境中，多个客户端同时调用 interval=None 会导致数值不准。
    # 推荐在主程序中使用单例/单线程负责定时采集，前端接口只负责读取缓存。
    return {
        "percent": psutil.cpu_percent(interval=None),
        "per_cpu": psutil.cpu_percent(interval=None, percpu=True),
        "phys_count": psutil.cpu_count(logical=False),
        "logical_count": psutil.cpu_count(logical=True),
        "freq": psutil.cpu_freq()._asdict() if psutil.cpu_freq() else None
    }


def collect_memory():
    mem = psutil.virtual_memory()
    return {
        "percent": mem.percent,
        "used": mem.used,
        "total": mem.total,
        "available": mem.available
    }


def collect_disk():
    partitions = []
    # all=False 可以在底层先帮我们过滤掉一部分无用的假分区
    for p in psutil.disk_partitions(all=False):
        # 过滤指定的文件系统类型
        if p.fstype in IGNORE_FSTYPES:
            continue
            
        # 过滤指定的设备前缀 (如 /dev/loopX)
        if any(p.device.startswith(prefix) for prefix in IGNORE_DEVICE_PREFIXES):
            continue

        try:
            usage = psutil.disk_usage(p.mountpoint)
            partitions.append({
                "device": p.device,
                "mount": p.mountpoint,
                "fstype": p.fstype,
                "percent": usage.percent,
                "used": usage.used,
                "total": usage.total
            })
        except PermissionError:
            # 遇到无权访问的挂载点（如 macOS 下的部分系统宗卷）直接跳过
            continue
            
    return partitions


def collect():
    return {
        "os": collect_os(),
        "cpu": collect_cpu(),
        "memory": collect_memory(),
        "disk": collect_disk()
    }