# scan_network.py
import subprocess
import sys
import re
import ipaddress
import concurrent.futures
import platform
import socket
import datetime

def get_gateway_and_ip():
    """自动获取本机IP和默认网关（支持 Windows 和 Linux）"""
    system = platform.system().lower()
    try:
        if system == "windows":
            return _get_gateway_ip_windows()
        elif system == "linux":
            return _get_gateway_ip_linux()
        else:
            print("⚠️  当前仅支持 Windows 和 Linux")
            return None, None
    except Exception as e:
        print(f"❌ 获取网络配置失败: {e}")
        return None, None

def _get_gateway_ip_windows():
    """Windows: 使用 ipconfig 获取网关和本机IP"""
    try:
        result = subprocess.run(["ipconfig"], capture_output=True, text=True, errors='ignore', timeout=10)
        output = result.stdout
        current_adapter = None
        for line in output.splitlines():
            if "无线局域网适配器 WLAN" in line or "以太网适配器" in line:
                current_adapter = line.strip()
            if current_adapter and "IPv4" in line:
                ip_match = re.search(r"IPv4 地址[\.\s]*: ([\d.]+)", line)
                gw_match = re.search(r"默认网关[\.\s]*: ([\d.]+)", line)
                if ip_match:
                    local_ip = ip_match.group(1)
                    gateway = gw_match.group(1) if gw_match else None
                    return local_ip, gateway
    except Exception as e:
        print(f"❌ Windows 获取网络配置失败: {e}")
    return None, None

def _get_gateway_ip_linux():
    """Linux: 使用 'ip route' 获取默认网关，并反推本机IP"""
    try:
        result = subprocess.run(["ip", "route", "show", "default"], capture_output=True, text=True, timeout=5)
        if result.returncode != 0 or not result.stdout.strip():
            return None, None
            
        default_line = result.stdout.strip().split('\n')[0]
        gw_match = re.search(r"via ([\d.]+)", default_line)
        dev_match = re.search(r"dev (\S+)", default_line)

        if not gw_match or not dev_match:
            return None, None

        gateway = gw_match.group(1)
        interface = dev_match.group(1)

        result_ip = subprocess.run(["ip", "addr", "show", interface], capture_output=True, text=True)
        ip_match = re.search(r"inet ([\d.]+/\d+)", result_ip.stdout)
        if not ip_match:
            return None, None

        local_ip = ip_match.group(1).split('/')[0]
        return local_ip, gateway
    except Exception as e:
        print(f"❌ Linux 获取网络配置失败: {e}")
        return None, None

def get_arp_table():
    """获取本机的 ARP 缓存表，用于提取 MAC 地址"""
    arp_table = {}
    try:
        system = platform.system().lower()
        cmd = ["arp", "-a"] if system == "windows" else ["arp", "-an"]
        result = subprocess.run(cmd, capture_output=True, text=True, errors='ignore')
        
        for line in result.stdout.splitlines():
            # 匹配 IP 地址
            ip_match = re.search(r"(\d+\.\d+\.\d+\.\d+)", line)
            # 匹配 MAC 地址 (支持横线或冒号分隔)
            mac_match = re.search(r"([0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2})", line)
            
            if ip_match and mac_match:
                ip = ip_match.group(1)
                mac = mac_match.group(1).replace('-', ':').upper()
                arp_table[ip] = mac
    except Exception:
        pass
    return arp_table

def ping_and_resolve(ip, timeout_ms=500, count=1):
    """Ping 单个IP，如果存活则尝试解析主机名"""
    ip_str = str(ip)
    is_alive = False
    hostname = "Unknown"
    
    try:
        if platform.system().lower() == "windows":
            cmd = ["ping", "-n", str(count), "-w", str(timeout_ms), ip_str]
        else:
            timeout_sec = max(1, timeout_ms // 1000)
            cmd = ["ping", "-c", str(count), "-W", str(timeout_sec), ip_str]

        result = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout_sec + 1 if platform.system().lower() != "windows" else 2)
        is_alive = result.returncode == 0
    except Exception:
        pass

    # 如果主机存活，尝试获取主机名
    if is_alive:
        try:
            # gethostbyaddr 会返回 (hostname, aliaslist, ipaddrlist)
            hostname, _, _ = socket.gethostbyaddr(ip_str)
        except socket.herror:
            pass # 无法解析时保持 "Unknown"
            
    return ip_str, is_alive, hostname

def scan_network(subnet=None, timeout_ms=500, max_threads=100):
    """扫描指定子网，多线程ping并解析信息"""
    if subnet is None:
        print("🔍 正在自动检测当前网络配置...")
        local_ip, gateway = get_gateway_and_ip()
        if gateway:
            net_ip = ".".join(gateway.split(".")[:3]) + ".0/24"
            try:
                subnet = ipaddress.IPv4Network(net_ip, strict=False)
                print(f"✅ 推测网段: {subnet}，网关: {gateway}")
            except Exception as e:
                print(f"❌ 网段解析失败: {e}，使用默认")
                subnet = ipaddress.IPv4Network("192.168.1.0/24")
        else:
            print("❌ 无法检测网段，使用默认 192.168.1.0/24")
            subnet = ipaddress.IPv4Network("192.168.1.0/24")

    print(f"\n🌐 开始扫描 {subnet}，超时={timeout_ms}ms，请稍候...\n")
    active_hosts = []

    # 多线程 Ping 扫描
    with concurrent.futures.ThreadPoolExecutor(max_workers=max_threads) as executor:
        futures = {executor.submit(ping_and_resolve, ip, timeout_ms): ip for ip in subnet.hosts()}

        for future in concurrent.futures.as_completed(futures):
            ip_str, is_alive, hostname = future.result()
            if is_alive:
                print(f"✅ 活跃: {ip_str:<15} | 主机名: {hostname}")
                active_hosts.append({
                    "ip": ip_str,
                    "hostname": hostname
                })

    # 按 IP 地址排序
    active_hosts.sort(key=lambda x: tuple(map(int, x["ip"].split('.'))))
    return active_hosts

def save_to_file(hosts_data, local_ip, gateway, filename="scan_results.txt"):
    """将扫描结果保存到 TXT 文件"""
    timestamp = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    try:
        with open(filename, "w", encoding="utf-8") as f:
            f.write(f"=== 局域网扫描报告 ===\n")
            f.write(f"扫描时间: {timestamp}\n")
            f.write(f"本机 IP: {local_ip if local_ip else '未知'}\n")
            f.write(f"网关 IP: {gateway if gateway else '未知'}\n")
            f.write("-" * 75 + "\n")
            f.write(f"{'IP 地址':<18} | {'MAC 地址':<19} | {'主机名/备注':<25}\n")
            f.write("-" * 75 + "\n")
            
            for host in hosts_data:
                ip = host["ip"]
                mac = host.get("mac", "Unknown")
                hostname = host.get("hostname", "Unknown")
                
                # 添加备注标识
                note = ""
                if ip == gateway:
                    note = " [网关]"
                elif ip == local_ip:
                    note = " [本机]"
                    
                display_name = f"{hostname}{note}"
                f.write(f"{ip:<18} | {mac:<19} | {display_name:<25}\n")
                
            f.write("-" * 75 + "\n")
            f.write(f"共发现 {len(hosts_data)} 台活跃设备。\n")
            
        print(f"\n📄 扫描结果已成功保存至: {filename}")
    except Exception as e:
        print(f"\n❌ 保存文件失败: {e}")

def main():
    print("🚀 局域网设备高级扫描工具\n")

    local_ip, gateway = get_gateway_and_ip()
    if local_ip:
        print(f"🏠 本机IP: {local_ip}")
    if gateway:
        print(f"🌐 默认网关: {gateway}")

    # 获取活跃主机和主机名
    active_hosts = scan_network(timeout_ms=500)

    # Ping 扫描结束后，操作系统的 ARP 缓存表会更新，此时读取 MAC 地址最准
    arp_table = get_arp_table()
    
    # 将 MAC 地址合并到结果中
    for host in active_hosts:
        host["mac"] = arp_table.get(host["ip"], "Unknown")

    print("\n" + "="*60)
    if active_hosts:
        print(f"✅ 发现 {len(active_hosts)} 台活跃设备:\n")
        print(f"{'IP 地址':<16} | {'MAC 地址':<18} | {'主机名'}")
        print("-" * 60)
        for host in active_hosts:
            ip_str = host["ip"]
            mark = ""
            if ip_str == gateway:
                mark = " (网关)"
            elif ip_str == local_ip:
                mark = " (本机)"
            
            print(f"{ip_str:<16} | {host['mac']:<18} | {host['hostname']}{mark}")
            
        # 调用保存文件函数
        save_to_file(active_hosts, local_ip, gateway)
    else:
        print("❌ 未发现任何活跃设备（可能是防火墙屏蔽了ICMP）")
        
if __name__ == "__main__":
    main()