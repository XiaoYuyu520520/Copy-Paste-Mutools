
# scan_network.py
import subprocess
import sys
import re
import ipaddress
import concurrent.futures
import platform

def get_gateway_and_ip():
    """自动获取本机IP和默认网关（仅Windows）"""
    if sys.platform != "win32":
        print("⚠️  当前仅支持 Windows 获取网关")
        return None, None

    try:
        result = subprocess.run(["ipconfig"], capture_output=True, text=True, encoding='gbk')  # Windows中文系统用gbk
        output = result.stdout

        # 查找当前活动的网络适配器（有IPv4地址）
        lines = output.splitlines()
        current_adapter = None
        for i, line in enumerate(lines):
            if "无线局域网适配器 WLAN" in line or "以太网适配器" in line:
                current_adapter = line.strip()
            if current_adapter and "IPv4" in line:
                ip_match = re.search(r"IPv4 地址[\.\s]*: ([\d.]+)", line)
                gw_match = re.search(r"默认网关[\.\s]*: ([\d.]+)", lines[i+4] if i+4 < len(lines) else "")
                if ip_match:
                    local_ip = ip_match.group(1)
                    gateway = gw_match.group(1) if gw_match else None
                    return local_ip, gateway
    except Exception as e:
        print(f"❌ 获取网络配置失败: {e}")
    return None, None

def ping_single_ip(ip, timeout_ms=500, count=1):
    """Ping 单个IP，返回是否存活"""
    try:
        if platform.system().lower() == "windows":
            cmd = ["ping", "-n", str(count), "-w", str(timeout_ms), str(ip)]
        else:
            cmd = ["ping", "-c", str(count), "-W", str(timeout_ms // 1000), str(ip)]

        result = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=2)
        return str(ip), result.returncode == 0
    except Exception as e:
        return str(ip), False

def scan_network(subnet=None, timeout_ms=500, max_threads=100):
    """扫描指定子网，多线程ping，返回活跃主机列表"""
    if subnet is None:
        print("🔍 正在自动检测当前网络配置...")
        _, gateway = get_gateway_and_ip()
        if gateway:
            # 推测 /24 网段
            net_ip = ".".join(gateway.split(".")[:3]) + ".0/24"
            subnet = ipaddress.IPv4Network(net_ip, strict=False)
            print(f"✅ 推测网段: {subnet}，网关: {gateway}")
        else:
            print("❌ 无法检测网段，使用默认 192.168.152.0/24")
            subnet = ipaddress.IPv4Network("192.168.152.0/24")

    print(f"\n🌐 开始扫描 {subnet}，超时={timeout_ms}ms，请稍候...\n")
    active_hosts = []

    with concurrent.futures.ThreadPoolExecutor(max_workers=max_threads) as executor:
        # 提交所有任务
        futures = {executor.submit(ping_single_ip, ip, timeout_ms): ip for ip in subnet.hosts()}
        
        for future in concurrent.futures.as_completed(futures):
            ip, is_alive = future.result()
            if is_alive:
                print(f"✅ {ip} —— 活跃")
                active_hosts.append(ip)
            else:
                # 可选：取消注释以显示离线
                # print(f"❌ {ip} —— 无响应", end='\r')
                pass

    active_hosts.sort(key=lambda ip: tuple(map(int, str(ip).split('.'))))
    return active_hosts

def main():
    print("🚀 局域网设备扫描工具（多线程）\n")

    # 获取网关和IP
    local_ip, gateway = get_gateway_and_ip()
    if local_ip:
        print(f"🏠 本机IP: {local_ip}")
    if gateway:
        print(f"🌐 默认网关: {gateway}")

    # 扫描
    active_hosts = scan_network(timeout_ms=500)

    print("\n" + "="*40)
    if active_hosts:
        print(f"✅ 发现 {len(active_hosts)} 台活跃设备:")
        for ip in active_hosts:
            mark = " ←←← 网关" if str(ip) == gateway else " ←←← 本机" if str(ip) == local_ip else ""
            print(f"  {ip}{mark}")
    else:
        print("❌ 未发现任何活跃设备（可能是防火墙屏蔽了ICMP）")

if __name__ == "__main__":
    main()