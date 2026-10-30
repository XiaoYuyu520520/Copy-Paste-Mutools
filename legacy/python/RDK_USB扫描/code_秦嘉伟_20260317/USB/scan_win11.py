import wmi
import pythoncom
import time

def get_usb_devices(wmi_client):
    """获取当前系统中的所有 USB 和相关串口设备"""
    devices = wmi_client.Win32_PnPEntity()
    # 过滤条件：保留 USB 总线、蓝牙或常见的串口芯片（如 FTDI）
    # 如果你想监听电脑上*所有*的设备变动，可以直接 return {d.DeviceID: d for d in devices if d.DeviceID}
    usb_devices = {}
    for d in devices:
        if d.DeviceID and ("USB" in d.DeviceID or "FTDIBUS" in d.DeviceID or "BTH" in d.DeviceID):
            usb_devices[d.DeviceID] = d
    return usb_devices

def monitor_usb_wmi():
    print("🚀 启动外设详细监听服务...")
    print("正在建立 WMI 连接，初始化设备快照 (可能需要几秒钟)...")
    
    # 初始化多线程 COM 环境
    pythoncom.CoInitialize()
    c = wmi.WMI()
    
    # 获取初始状态
    known_devices = get_usb_devices(c)
    print(f"初始化完成，当前已连接 {len(known_devices)} 个相关外设。")
    print("请插入或拔出设备 (按 Ctrl+C 退出)。\n")

    try:
        while True:
            time.sleep(1)  # 轮询间隔，1秒能平衡响应速度和 CPU 占用
            current_devices = get_usb_devices(c)
            
            # 1. 寻找新插入的设备 (差集)
            added_ids = set(current_devices.keys()) - set(known_devices.keys())
            for dev_id in added_ids:
                dev = current_devices[dev_id]
                print(f"\n[+] 🟢 发现新外设插入:")
                print(f"    - 设备名称: {dev.Name}")
                print(f"    - 制造商:   {dev.Manufacturer if dev.Manufacturer else '未知'}")
                print(f"    - 硬件 ID:  {dev.DeviceID}")
                print(f"    - 设备描述: {dev.Description}")

            # 2. 寻找拔出的设备 (差集)
            removed_ids = set(known_devices.keys()) - set(current_devices.keys())
            for dev_id in removed_ids:
                dev = known_devices[dev_id]
                print(f"\n[-] 🔴 检测到外设拔出:")
                print(f"    - 设备名称: {dev.Name}")
                print(f"    - 硬件 ID:  {dev.DeviceID}")
            
            # 更新快照
            known_devices = current_devices

    except KeyboardInterrupt:
        print("\n监听已停止。")

if __name__ == "__main__":
    monitor_usb_wmi()