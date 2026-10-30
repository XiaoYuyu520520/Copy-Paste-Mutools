import os
import warnings
import subprocess
import shutil

# ==========================================
# 配置参数
# ==========================================
# 桌面端多显卡环境下，默认采集的 GPU 索引 (-1 表示返回所有显卡的列表)
TARGET_GPU_INDEX = 0
# ==========================================

# 全局状态缓存
GPU_PLATFORM = None  # 'desktop_nvidia', 'jetson', None
_JTOP_INSTANCE = None
_NVML_INITIALIZED = False


def _detect_platform():
    global GPU_PLATFORM
    if GPU_PLATFORM is not None:
        return GPU_PLATFORM

    # 1) Check Jetson via jtop import
    try:
        from jtop import jtop
        # 仅作探测，不保持连接
        with jtop() as jetson:
            if jetson.ok():
                GPU_PLATFORM = "jetson"
                return GPU_PLATFORM
    except Exception:
        pass

    # 2) Check Jetson via sysfs
    try:
        with open("/sys/devices/gpu.0/load", "r") as f:
            _ = f.read()
        GPU_PLATFORM = "jetson"
        return GPU_PLATFORM
    except (FileNotFoundError, PermissionError, OSError):
        pass

    # 3) Check desktop NVIDIA via nvidia-smi
    if shutil.which("nvidia-smi"):
        GPU_PLATFORM = "desktop_nvidia"
        return GPU_PLATFORM

    # 4) Try pynvml import
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("ignore", FutureWarning)
            import pynvml
            pynvml.nvmlInit()
            pynvml.nvmlShutdown()
        GPU_PLATFORM = "desktop_nvidia"
        return GPU_PLATFORM
    except Exception:
        pass

    GPU_PLATFORM = None
    return None


def _get_jtop_instance():
    """管理 jtop 单例，避免高频创建销毁"""
    global _JTOP_INSTANCE
    if _JTOP_INSTANCE is None:
        try:
            from jtop import jtop
            _JTOP_INSTANCE = jtop()
            _JTOP_INSTANCE.start()
        except Exception:
            return None
    return _JTOP_INSTANCE


def _collect_jetson():
    jetson = _get_jtop_instance()
    
    if not jetson or not jetson.ok():
        return _collect_jetson_fallback()

    try:
        gpu_raw = jetson.gpu
        temps = jetson.temperature

        result = {
            "platform": "jetson",
            "name": "NVIDIA Tegra (Jetson)",
            "valid": True
        }

        if gpu_raw and 'gpu' in gpu_raw:
            info = gpu_raw['gpu']
            cur_freq_khz = info.get('freq', {}).get('cur', 0)
            max_freq_khz = info.get('freq', {}).get('max', 1)
            result["freq_mhz"] = round(cur_freq_khz / 1000, 1)
            result["freq_max_mhz"] = round(max_freq_khz / 1000, 1)
            result["freq_pct"] = round(cur_freq_khz / max(1, max_freq_khz) * 100, 1)

        if temps:
            gpu_temp = temps.get('gpu', {})
            if isinstance(gpu_temp, dict):
                t = gpu_temp.get('temp', None)
            else:
                t = gpu_temp
            if t is not None:
                result["temp"] = round(t, 1)

        mem = jetson.memory
        if mem and 'RAM' in mem:
            ram = mem['RAM']
            used_kb = ram.get('used', 0)
            total_kb = ram.get('tot', 1)
            result["mem_percent"] = round(used_kb / total_kb * 100, 1)
            result["mem_used"] = int(used_kb * 1024)
            result["mem_total"] = int(total_kb * 1024)

        return result

    except Exception:
        return _collect_jetson_fallback()


def _collect_jetson_fallback():
    result = {
        "platform": "jetson",
        "name": "NVIDIA Tegra (Jetson)",
        "valid": True,
        "percent": 0,
        "temp": 0,
        "freq_mhz": 0
    }

    # 读取利用率 (0-1000 对应 0.0% - 100.0%)
    try:
        with open("/sys/devices/gpu.0/load", "r") as f:
            raw = f.read().strip()
            result["percent"] = int(raw) // 10
    except Exception:
        pass

    # 修复：正确读取 thermal zone 的 type 来判断是否是 GPU
    try:
        thermal_dir = "/sys/class/thermal/"
        for zone in os.listdir(thermal_dir):
            if zone.startswith("thermal_zone"):
                type_path = os.path.join(thermal_dir, zone, "type")
                temp_path = os.path.join(thermal_dir, zone, "temp")
                
                if os.path.exists(type_path) and os.path.exists(temp_path):
                    with open(type_path, "r") as f:
                        zone_type = f.read().strip().lower()
                    
                    if "gpu" in zone_type:
                        with open(temp_path, "r") as f:
                            result["temp"] = int(f.read().strip()) // 1000
                        break
    except Exception:
        pass

    return result


def _init_nvml():
    """安全初始化 NVML 单例"""
    global _NVML_INITIALIZED
    if not _NVML_INITIALIZED:
        try:
            with warnings.catch_warnings():
                warnings.simplefilter("ignore", FutureWarning)
                import pynvml
                pynvml.nvmlInit()
                _NVML_INITIALIZED = True
        except Exception:
            pass
    return _NVML_INITIALIZED


def _collect_desktop_nvidia():
    if not _init_nvml():
        return None

    try:
        import pynvml
        count = pynvml.nvmlDeviceGetCount()
        gpus = []

        for i in range(count):
            handle = pynvml.nvmlDeviceGetHandleByIndex(i)
            name = pynvml.nvmlDeviceGetName(handle)
            util = pynvml.nvmlDeviceGetUtilizationRates(handle)
            mem_info = pynvml.nvmlDeviceGetMemoryInfo(handle)
            temp = pynvml.nvmlDeviceGetTemperature(handle, pynvml.NVML_TEMPERATURE_GPU)

            gpus.append({
                "platform": "desktop_nvidia",
                "name": name,
                "valid": True,
                "percent": util.gpu,
                "mem_percent": round(mem_info.used / mem_info.total * 100, 1),
                "mem_used": mem_info.used,
                "mem_total": mem_info.total,
                "temp": temp,
            })

        if not gpus:
            return None
            
        # 根据配置参数决定返回单卡还是所有卡
        if TARGET_GPU_INDEX == -1:
            return gpus
        elif TARGET_GPU_INDEX < len(gpus):
            return gpus[TARGET_GPU_INDEX]
        else:
            return gpus[0]  # 越界回退到第一张卡

    except Exception:
        return None


def collect():
    platform_type = _detect_platform()

    if platform_type == "jetson":
        return _collect_jetson()
    elif platform_type == "desktop_nvidia":
        return _collect_desktop_nvidia()

    return None

# 如果需要在主程序退出时清理，可以暴露一个 cleanup 方法
def cleanup():
    global _JTOP_INSTANCE, _NVML_INITIALIZED
    if _JTOP_INSTANCE is not None:
        _JTOP_INSTANCE.close()
    if _NVML_INITIALIZED:
        import pynvml
        pynvml.nvmlShutdown()
        _NVML_INITIALIZED = False