# main.py
import pyudev
import time
import datetime
import sys
from config import LOG_FILE, TIME_FORMAT, MD_SCAN_HEADER, MD_MONITOR_HEADER, POLL_TIMEOUT

def get_current_time():
    """获取格式化后的当前时间"""
    return datetime.datetime.now().strftime(TIME_FORMAT)

def write_to_log(content):
    """将内容写入 Markdown 日志文件"""
    try:
        with open(LOG_FILE, "a", encoding="utf-8") as f:
            f.write(content + "\n")
    except Exception as e:
        print(f"写入日志失败: {e}")

def get_device_info(device):
    """解析 udev 设备对象，提取有用信息"""
    vendor = device.get('ID_VENDOR_FROM_DATABASE') or device.get('ID_VENDOR') or 'Unknown'
    product = device.get('ID_MODEL_FROM_DATABASE') or device.get('ID_MODEL') or 'Unknown'
    dev_node = device.device_node or 'N/A'
    sys_path = device.sys_path or 'N/A'
    
    return vendor, product, dev_node, sys_path

def scan_usb():
    """功能2：单次扫描已连接的USB设备"""
    print("\n" + "="*50)
    print("开始扫描当前已连接的USB设备...")
    print("="*50)
    
    context = pyudev.Context()
    # 仅请求 usb 子系统
    devices = context.list_devices(subsystem='usb')
    
    current_time = get_current_time()
    log_content = f"\n### USB 设备单次扫描报告 ({current_time})\n\n{MD_SCAN_HEADER}\n"
    
    count = 0
    for device in devices:
        # 在 Python 端过滤，只保留主设备(usb_device)，过滤掉子接口(usb_interface)
        if device.get('DEVTYPE') != 'usb_device':
            continue
            
        vendor, product, dev_node, sys_path = get_device_info(device)
        
        # 打印到控制台
        print(f"[{count+1}] 厂商: {vendor} | 产品: {product}")
        print(f"    设备路径: {dev_node}")
        print(f"    系统路径: {sys_path}\n")
        
        # 格式化为 Markdown 表格行
        log_content += f"| {current_time} | {vendor} | {product} | `{dev_node}` | `{sys_path}` |\n"
        count += 1
        
    if count == 0:
        print("未检测到任何 USB 设备。")
        log_content += "| - | 无 | 无 | 无 | 无 |\n"
        
    write_to_log(log_content)
    print(f"扫描完成，共找到 {count} 个设备。结果已追加至 {LOG_FILE}\n")

def monitor_usb():
    """功能1：监听USB设备的插拔变动"""
    print("\n" + "="*50)
    print("进入 USB 监听模式 (按 Ctrl+C 退出并返回主菜单)...")
    print("="*50)
    
    context = pyudev.Context()
    monitor = pyudev.Monitor.from_netlink(context)
    # 仅过滤 usb 子系统，解决参数报错问题
    monitor.filter_by(subsystem='usb')
    
    start_time = get_current_time()
    log_content = f"\n### USB 监听会话开始 ({start_time})\n\n{MD_MONITOR_HEADER}\n"
    write_to_log(log_content)
    
    try:
        # 启动监听循环
        for device in iter(monitor.poll, None):
            # 同样在 Python 端过滤掉 usb_interface 的冗余信息
            if device.get('DEVTYPE') != 'usb_device':
                continue
                
            action = device.action
            if action not in ['add', 'remove']:
                continue
                
            current_time = get_current_time()
            vendor, product, dev_node, sys_path = get_device_info(device)
            
            # 控制台输出带有颜色的动作提示（绿底加，红底减）
            action_str = "\033[92m[插入 ADD]\033[0m" if action == 'add' else "\033[91m[拔出 REMOVE]\033[0m"
            md_action = "**插入 (ADD)**" if action == 'add' else "**拔出 (REMOVE)**"
            
            print(f"{action_str} 时间: {current_time}")
            print(f"    厂商: {vendor} | 产品: {product}")
            print(f"    设备路径: {dev_node}")
            print(f"    系统路径: {sys_path}\n")
            
            # 实时写入单条日志
            log_line = f"| {md_action} | {current_time} | {vendor} | {product} | `{dev_node}` | `{sys_path}` |"
            write_to_log(log_line)
            
    except KeyboardInterrupt:
        end_time = get_current_time()
        print("\n\n退出监听模式...")
        write_to_log(f"\n*监听会话结束于 {end_time}*\n---")

def main():
    """主菜单"""
    while True:
        print("="*30)
        print("    Ubuntu USB 管理工具")
        print("="*30)
        print("1. 监听模式 (实时打印USB插拔及路径)")
        print("2. 单次扫描 (列出当前连接的USB设备)")
        print("3. 退出程序")
        print("="*30)
        
        choice = input("请输入选项 (1/2/3): ").strip()
        
        if choice == '1':
            monitor_usb()
        elif choice == '2':
            scan_usb()
        elif choice == '3':
            print("退出程序。Bye!")
            sys.exit(0)
        else:
            print("无效选项，请重新输入。\n")

if __name__ == "__main__":
    # 如果没有生成过日志文件，先初始化一个标题
    try:
        with open(LOG_FILE, "r") as f:
            pass
    except FileNotFoundError:
        write_to_log("# Ubuntu USB 设备活动日志\n")
        
    main()