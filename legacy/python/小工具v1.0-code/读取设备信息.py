# 文件名: get_device_info_cli.py
# 修正了WMI笔误和补充了缺失的打印逻辑

import platform
import psutil
import wmi
import os
import sys
import json

try:
    import GPUtil
except ImportError:
    GPUtil = None

# ==============================================================================
#  第一部分: “返回数据”的函数 (get_xxx_as_dict)
# ==============================================================================

def get_system_info_as_dict():
    """返回包含系统信息的字典"""
    try:
        uname = platform.uname()
        c = wmi.WMI()
        ### 修正 ###: 将 Win2_ 改为 Win32_
        cs_info = c.Win32_ComputerSystem()[0]
        os_info = c.Win32_OperatingSystem()[0]
        return {
            "os": uname.system,
            "node_name": uname.node,
            "release": uname.release,
            "version": uname.version,
            "architecture": uname.machine,
            "manufacturer": cs_info.Manufacturer,
            "model": cs_info.Model,
            "system_type": cs_info.SystemType,
            "last_boot_time": os_info.LastBootUpTime,
        }
    except Exception as e:
        return {"error": str(e)}

def get_cpu_info_as_dict():
    """返回包含CPU信息的字典"""
    try:
        return {
            "name": platform.processor(),
            "physical_cores": psutil.cpu_count(logical=False),
            "logical_cores": psutil.cpu_count(logical=True),
            "usage_percent": psutil.cpu_percent(interval=1)
        }
    except Exception as e:
        return {"error": str(e)}

def get_gpu_info_as_dict():
    """返回包含所有GPU信息的列表"""
    gpus_list = []
    if GPUtil:
        try:
            gpus = GPUtil.getGPUs()
            for gpu in gpus:
                gpus_list.append({
                    "source": "GPUtil",
                    "name": gpu.name,
                    "memory_total_mb": gpu.memoryTotal,
                    "memory_used_mb": gpu.memoryUsed,
                    "usage_percent": gpu.load * 100,
                    "temperature_celsius": gpu.temperature
                })
        except Exception:
            pass
    try:
        c = wmi.WMI()
        video_controllers = c.Win32_VideoController()
        for controller in video_controllers:
            if any(controller.Name in gpu_info.get("name", "") for gpu_info in gpus_list):
                continue
            ram_bytes = int(controller.AdapterRAM) if controller.AdapterRAM and int(controller.AdapterRAM) > 0 else None
            gpus_list.append({
                "source": "WMI",
                "name": controller.Name,
                "memory_total_bytes": ram_bytes,
                "driver_version": controller.DriverVersion,
            })
    except Exception as e:
        if not gpus_list:
            return {"error": str(e)}
    return {"gpus": gpus_list}

def get_memory_info_as_dict():
    """返回内存信息字典"""
    try:
        svmem = psutil.virtual_memory()
        return {
            "total_bytes": svmem.total,
            "available_bytes": svmem.available,
            "used_bytes": svmem.used,
            "usage_percent": svmem.percent
        }
    except Exception as e:
        return {"error": str(e)}

def get_disk_info_as_dict():
    """返回磁盘信息列表"""
    disks = []
    try:
        partitions = psutil.disk_partitions()
        for p in partitions:
            try:
                usage = psutil.disk_usage(p.mountpoint)
                disks.append({
                    "device": p.device,
                    "mountpoint": p.mountpoint,
                    "fstype": p.fstype,
                    "total_bytes": usage.total,
                    "used_bytes": usage.used,
                    "free_bytes": usage.free,
                    "usage_percent": usage.percent
                })
            except Exception:
                continue
        return {"disks": disks}
    except Exception as e:
        return {"error": str(e)}

def get_all_info_as_dict():
    """返回一个包含所有设备信息的巨大字典"""
    return {
        "system": get_system_info_as_dict(),
        "cpu": get_cpu_info_as_dict(),
        "gpu": get_gpu_info_as_dict(),
        "memory": get_memory_info_as_dict(),
        "disk": get_disk_info_as_dict(),
    }

# ==============================================================================
#  第二部分: 格式化打印函数
# ==============================================================================
def print_all_info_for_human():
    """为人类用户打印所有信息"""
    
    # 打印系统信息
    print("="*40, "系统信息", "="*40)
    system_info = get_system_info_as_dict()
    if "error" in system_info:
        print(f"获取系统信息失败: {system_info['error']}")
    else:
        for k, v in system_info.items(): print(f"{k.replace('_', ' ').capitalize()}: {v}")
    
    # 打印CPU信息
    print("\n" + "="*40, "CPU 信息", "="*40)
    cpu_info = get_cpu_info_as_dict()
    for k, v in cpu_info.items(): print(f"{k.replace('_', ' ').capitalize()}: {v}")

    # 打印GPU信息
    print("\n" + "="*40, "显卡信息", "="*40)
    gpu_info = get_gpu_info_as_dict()
    for i, gpu in enumerate(gpu_info.get("gpus", [])):
        print(f"--- 显卡 {i} ({gpu.get('source')}) ---")
        for k, v in gpu.items(): print(f"  {k.replace('_', ' ').capitalize()}: {v}")
    
    ### 修正 ###: 补充内存和磁盘的打印逻辑
    # 打印内存信息
    print("\n" + "="*40, "内存信息", "="*40)
    memory_info = get_memory_info_as_dict()
    # 为了更好的可读性，我们对字节数进行格式化
    print(f"Total bytes: {memory_info.get('total_bytes')} ({psutil._common.bytes2human(memory_info.get('total_bytes', 0))})")
    print(f"Available bytes: {memory_info.get('available_bytes')} ({psutil._common.bytes2human(memory_info.get('available_bytes', 0))})")
    print(f"Used bytes: {memory_info.get('used_bytes')} ({psutil._common.bytes2human(memory_info.get('used_bytes', 0))})")
    print(f"Usage percent: {memory_info.get('usage_percent')}%")

    # 打印磁盘信息
    print("\n" + "="*40, "磁盘信息", "="*40)
    disk_info = get_disk_info_as_dict()
    for i, disk in enumerate(disk_info.get("disks", [])):
        print(f"--- 磁盘 {i} ({disk.get('device')}) ---")
        print(f"  Mountpoint: {disk.get('mountpoint')}")
        print(f"  Fstype: {disk.get('fstype')}")
        print(f"  Total: {psutil._common.bytes2human(disk.get('total_bytes', 0))}")
        print(f"  Used: {psutil._common.bytes2human(disk.get('used_bytes', 0))}")
        print(f"  Free: {psutil._common.bytes2human(disk.get('free_bytes', 0))}")
        print(f"  Usage percent: {disk.get('usage_percent')}%")
    
    input("\n信息获取完毕，请按回车键退出...")

# ==============================================================================
#  第三部分: 主逻辑 (Main)
# ==============================================================================
def main():
    """程序主入口"""
    args = sys.argv[1:]
    
    if not args:
        print_all_info_for_human()
        return

    output_data = {}
    if "--json" in args:
        try:
            query_index = args.index("--json") + 1
            if query_index < len(args):
                query = args[query_index]
                if query == "cpu":
                    output_data = get_cpu_info_as_dict()
                elif query == "system":
                    output_data = get_system_info_as_dict()
                elif query == "gpu":
                    output_data = get_gpu_info_as_dict()
                elif query == "memory":
                    output_data = get_memory_info_as_dict()
                elif query == "disk":
                    output_data = get_disk_info_as_dict()
                elif query == "all":
                    output_data = get_all_info_as_dict()
                else:
                    output_data = {"error": f"Unknown query target: {query}"}
            else:
                output_data = get_all_info_as_dict() # 如果只提供了--json，则返回全部信息
        except Exception as e:
            output_data = {"error": str(e)}
        print(json.dumps(output_data, indent=2))
    else:
        print("用法:")
        print("  直接运行:                  进入交互模式，显示所有信息。")
        print("  --json <target>:         以JSON格式输出指定信息。")
        print("  <target> 可选项: system, cpu, gpu, memory, disk, all (默认为 all)")

if __name__ == "__main__":
    main()