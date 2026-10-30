
# scan_network.py
import subprocess
import sys
import re
import ipaddress
import concurrent.futures
import platform
import socket
import struct


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
        result = subprocess.run(["ipconfig"], capture_output=True, text=True, encoding='gbk', timeout=10)
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
        # 获取默认路由
        result = subprocess.run(["ip", "route", "show", "default"], capture_output=True, text=True, timeout=5)
        if result.returncode != 0 or not result.stdout.strip():
            return None, None

        # 解析默认网关和出口接口
        # 示例: default via 192.168.152.1 dev wlP1p1s0 proto dhcp metric 600
        default_line = result.stdout.strip().split('\n')[0]
        gw_match = re.search(r"via ([\d.]+)", default_line)
        dev_match = re.search(r"dev (\S+)", default_line)

        if not gw_match or not dev_match:
            return None, None

        gateway = gw_match.group(1)
        interface = dev_match.group(1)

        # 获取该接口的IP地址
        result_ip = subprocess.run(["ip", "addr", "show", interface], capture_output=True, text=True)
        ip_output = result_ip.stdout

        # 查找 inet 地址（排除 127.0.0.1）
        ip_match = re.search(r"inet ([\d.]+/\d+)", ip_output)
        if not ip_match:
            return None, None

        cidr_ip = ip_match.group(1)
        # 提取纯IP（如 192.168.152.119）
        local_ip = cidr_ip.split('/')[0]

        return local_ip, gateway

    except Exception as e:
        print(f"❌ Linux 获取网络配置失败: {e}")
        return None, None


def ping_single_ip(ip, timeout_ms=500, count=1):
    """Ping 单个IP，返回是否存活"""
    try:
        if platform.system().lower() == "windows":
            cmd = ["ping", "-n", str(count), "-w", str(timeout_ms), str(ip)]
        else:
            # 使用 -W 秒（非毫秒），所以转换
            timeout_sec = max(1, timeout_ms // 1000)
            cmd = ["ping", "-c", str(count), "-W", str(timeout_sec), str(ip)]

        result = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout_sec + 1)
        return str(ip), result.returncode == 0
    except Exception as e:
        return str(ip), False


def scan_network(subnet=None, timeout_ms=500, max_threads=100):
    """扫描指定子网，多线程ping，返回活跃主机列表"""
    if subnet is None:
        print("🔍 正在自动检测当前网络配置...")
        local_ip, gateway = get_gateway_and_ip()
        if gateway:
            # 推测 /24 网段
            net_ip = ".".join(gateway.split(".")[:3]) + ".0/24"
            try:
                subnet = ipaddress.IPv4Network(net_ip, strict=False)
                print(f"✅ 推测网段: {subnet}，网关: {gateway}")
            except Exception as e:
                print(f"❌ 网段解析失败: {e}，使用默认")
                subnet = ipaddress.IPv4Network("192.168.152.0/24")
        else:
            print("❌ 无法检测网段，使用默认 192.168.152.0/24")
            subnet = ipaddress.IPv4Network("192.168.152.0/24")

    print(f"\n🌐 开始扫描 {subnet}，超时={timeout_ms}ms，请稍候...\n")
    active_hosts = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_threads) as executor:
        futures = {executor.submit(ping_single_ip, ip, timeout_ms): ip for ip in subnet.hosts()}

        for future in concurrent.futures.as_completed(futures):
            ip, is_alive = future.result()
            if is_alive:
                print(f"✅ {ip} —— 活跃")
                active_hosts.append(ip)

    active_hosts.sort(key=lambda ip: tuple(map(int, str(ip).split('.'))))
    return active_hosts


def main():
    print("🚀 局域网设备扫描工具（多线程）\n")

    local_ip, gateway = get_gateway_and_ip()
    if local_ip:
        print(f"🏠 本机IP: {local_ip}")
    if gateway:
        print(f"🌐 默认网关: {gateway}")

    active_hosts = scan_network(timeout_ms=500)

    print("\n" + "="*40)
    if active_hosts:
        print(f"✅ 发现 {len(active_hosts)} 台活跃设备:")
        for ip in active_hosts:
            ip_str = str(ip)
            mark = ""
            if ip_str == gateway:
                mark = " ←←← 网关"
            elif ip_str == local_ip:
                mark = " ←←← 本机"
            print(f"  {ip_str}{mark}")
    else:
        print("❌ 未发现任何活跃设备（可能是防火墙屏蔽了ICMP）")


if __name__ == "__main__":
    main()