import subprocess
import time
import sys

# ================= 配置参数 =================
# 监控的挂载点路径
MOUNTPOINT = "/mnt/2t-hdd"
# ============================================

def get_disk_from_mountpoint(mountpoint):
    """自动获取挂载点对应的磁盘名（如 sda、sdb、nvme0n1）"""
    try:
        # 使用 lsblk 获取干净的映射关系
        result = subprocess.run(["lsblk", "-no", "PKNAME,TYPE,MOUNTPOINT"], capture_output=True, text=True)
        for line in result.stdout.splitlines():
            if mountpoint in line and ("part" in line or "disk" in line):
                # 提取主设备名（例如从 sda1 提取 sda）
                return line.split()[0]
    except Exception as e:
        print(f"[-] 获取磁盘映射失败: {e}")
        pass
    # 默认兜底返回 sda
    return "sda"

def monitor_disk():
    device = get_disk_from_mountpoint(MOUNTPOINT)
    print(f"✅ 实时监控磁盘: {device} ({MOUNTPOINT})")
    print("=" * 55)
    print(f"{'时间':<10} {'读MB/s':<12} {'写MB/s':<12} {'磁盘繁忙率':<12}")
    print("=" * 55)

    # 核心优化：使用 Popen 开启一个持续运行的后台进程，而不是放在 while 循环里反复创建
    cmd = ["iostat", "-x", "-m", "1", device]
    
    try:
        # stdout=subprocess.PIPE 允许我们实时逐行读取输出
        process = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        
        # 初始化列索引
        idx_rMB = idx_wMB = idx_util = -1
        is_first_report = True
        
        for line in process.stdout:
            line = line.strip()
            if not line:
                continue
                
            # 1. 动态抓取表头，获取各项指标的正确下标 (解决 iostat 版本兼容性问题)
            if line.startswith("Device"):
                headers = line.split()
                try:
                    idx_rMB = headers.index("rMB/s")
                    idx_wMB = headers.index("wMB/s")
                    idx_util = headers.index("%util")
                except ValueError:
                    pass # 如果因特殊原因找不到表头，保留 -1
                continue
            
            # 2. 匹配目标设备数据行
            if line.startswith(device):
                # iostat 的第一组数据是自系统启动以来的历史平均值，不能反映瞬时 IO，需要跳过
                if is_first_report:
                    is_first_report = False
                    continue

                parts = line.split()
                
                # 如果成功解析了表头，直接通过下标精准取值
                if idx_rMB != -1 and idx_wMB != -1 and idx_util != -1:
                    rMB = parts[idx_rMB]
                    wMB = parts[idx_wMB]
                    util = parts[idx_util]
                else:
                    # 兜底机制：如果没有抓到表头，则使用一个经验位置（容错）
                    rMB = parts[2] if len(parts) > 2 else "0.00"
                    wMB = parts[3] if len(parts) > 3 else "0.00"
                    util = parts[-1] if len(parts) > 0 else "0.00"

                now = time.strftime("%H:%M:%S")
                # 使用 \r 覆盖同一行输出，保持界面整洁
                print(f"\r{now:<10} {rMB:<12} {wMB:<12} {util:<12}", end="")
                sys.stdout.flush()

    except KeyboardInterrupt:
        print("\n\n🛑 监控停止")
    except Exception as e:
        print(f"\n\n[-] 发生错误: {e}")
    finally:
        # 退出时确保清理掉后台的 iostat 进程
        if 'process' in locals():
            process.terminate()

if __name__ == "__main__":
    monitor_disk()