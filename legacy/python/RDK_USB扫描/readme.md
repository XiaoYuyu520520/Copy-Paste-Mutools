在Ubuntu（Linux）环境下，处理USB设备最标准且高效的Python库是 pyudev（Linux udev 的Python绑定）。它可以精准地获取USB设备的详细信息（包括设备路径 /dev/... 和系统路径 /sys/...），并且能实时监听设备的插拔事件。

下面是为你编写的两个Python脚本：config.py 和 main.py。
1. 环境准备

在Ubuntu上，你需要安装 pyudev 库。打开终端运行：
BASH

sudo apt update
sudo apt install python3-pip
pip3 install pyudev

(注意：为了获取完整的底层设备信息，建议运行时使用 sudo)
2. 配置文件：config.py

这个文件用于存放默认参数，方便随时修改日志文件名或时间格式。
PYTHON

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

3. 主程序：main.py

将以下代码保存为 main.py，与 config.py 放在同一个目录下。
PYTHON

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
    # 过滤出 USB 设备
    devices = context.list_devices(subsystem='usb', devtype='usb_device')
    
    current_time = get_current_time()
    log_content = f"\n### USB 设备单次扫描报告 ({current_time})\n\n{MD_SCAN_HEADER}\n"
    
    count = 0
    for device in devices:
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
    monitor.filter_by(subsystem='usb', devtype='usb_device')
    
    start_time = get_current_time()
    log_content = f"\n### USB 监听会话开始 ({start_time})\n\n{MD_MONITOR_HEADER}\n"
    write_to_log(log_content)
    
    try:
        # 启动监听循环
        for device in iter(monitor.poll, None):
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

4. 运行说明

    确保 config.py 和 main.py 在同一个文件夹中。
    在终端中运行脚本（强烈建议加上 sudo，否则可能无法读取部分底层的 /dev/ 和 /sys/ 信息）：
    BASH

    sudo python3 main.py

    操作体验：
        选择 2：会瞬间列出鼠标、键盘、U盘、内置蓝牙模块等所有USB设备，并排版好写入 usb_log.md。
        选择 1：屏幕挂起。此时你拔插一个U盘或USB鼠标，屏幕会立刻打印出动作（Add/Remove），并且附带具体的设备挂载点（如 /dev/bus/usb/001/005），同时追加写入 usb_log.md。
        在模式 1 下，按下 Ctrl + C 会安全退出监听并返回主菜单。
    日志查看：退出后或者在另一个终端运行 cat usb_log.md（或者用Markdown编辑器打开），你可以看到非常漂亮的表格格式的记录。
这是一个非常经典的 Python 虚拟环境与 sudo 权限冲突的问题。
为什么会报错？

从你的终端提示可以看出：

    你当前处在一个名为 (ai_lab) 的虚拟环境中。
    你运行 pip3 install pyudev 时，它被安装到了虚拟环境里（路径是 /home/ubuntu/桌面/environment/ai_lab/...）。
    但是，当你运行 sudo python3 main.py 时，sudo 会切换到 root 用户的系统级 Python (/usr/bin/python3)。系统的 Python 并没有安装 pyudev，也无法读取你虚拟环境里的库，所以报了 ModuleNotFoundError。

解决方法

这里提供两种最常用的解决办法，任选其一即可：
方法一：使用虚拟环境的 Python 解释器运行（推荐）

既然库安装在虚拟环境中，我们可以直接告诉 sudo 去调用虚拟环境里的 python3。
根据你终端输出的路径，你的虚拟环境 Python 路径应该是 /home/ubuntu/桌面/environment/ai_lab/bin/python3。

请运行以下命令：
BASH

sudo /home/ubuntu/桌面/environment/ai_lab/bin/python3 main.py

(这样既拿到了 root 权限扫描底层 USB 信息，又成功使用了虚拟环境里的 pyudev 库)
方法二：直接在系统级安装 pyudev

如果你希望以后直接敲 sudo python3 main.py 就能运行，你需要把 pyudev 安装到系统环境里。
在 Ubuntu 中，由于现在限制了 sudo pip3 的使用，最标准的方法是使用 apt 安装系统级的 Python 依赖库：

请依次运行：
BASH

sudo apt update
sudo apt install python3-pyudev

安装完成后，你原本的命令就可以正常运行了：
BASH

sudo python3 main.py

提示：运行成功后，你可以先尝试拔插一下鼠标或键盘，测试一下监听功能（选项1）的效果。日志会自动写入当前目录的 usb_log.md 中。

这两个问题是由于 pyudev 库的版本参数差异导致的。

    报错原因（选项 1）：Monitor.filter_by() 不接受 devtype 作为关键字参数，正确的参数名应该是 device_type，或者干脆只过滤 subsystem。
    扫描为空原因（选项 2）：在 list_devices() 中使用 devtype='usb_device' 过滤条件过于严苛，导致把所有设备都默默过滤掉了（实际上即使你不插外部U盘，笔记本也会有内置的USB设备，如蓝牙、摄像头、USB Hub等）。

为了保证代码在任何版本的 pyudev 下都能 100% 稳定运行，最稳妥的做法是：只向底层请求 subsystem='usb' 的设备，然后在 Python 代码里通过 device.get('DEVTYPE') == 'usb_device' 进行精准筛选。

请用以下代码完全替换你的 main.py（config.py 不变）：
PYTHON

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

修改点说明：

    去掉了报错的参数：把 monitor.filter_by(subsystem='usb', devtype='usb_device') 改为了 monitor.filter_by(subsystem='usb')，彻底避免 TypeError。
    修复了0个设备的问题：使用 device.get('DEVTYPE') != 'usb_device' 进行逻辑过滤。这样当你按 2 时，就能准确扫出你笔记本上的 USB 根集线器、内置摄像头（如果有）、内置蓝牙模块等真实物理设备了。

保存后继续执行刚才的命令即可：
BASH

sudo /home/ubuntu/桌面/environment/ai_lab/bin/python3 main.py
sudo /home/ubuntu/ai_lab/bin/python3 main.py


