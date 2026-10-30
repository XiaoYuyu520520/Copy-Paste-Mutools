import serial
import time
import binascii

def sniff_lidar_data(com_port="COM3", baud_rate=115200):
    print(f"🔄 正在尝试连接雷达 ({com_port} @ {baud_rate} bps)...")
    
    try:
        # 初始化串口
        ser = serial.Serial(
            port=com_port,
            baudrate=baud_rate,
            parity=serial.PARITY_NONE,
            stopbits=serial.STOPBITS_ONE,
            bytesize=serial.EIGHTBITS,
            timeout=1  # 1秒超时
        )
        print("✅ 串口连接成功！正在读取原始数据...\n")
        print("-" * 50)
        
        while True:
            # 等待缓冲区有数据
            if ser.in_waiting > 0:
                # 一次性读取缓冲区内的所有数据
                raw_data = ser.read(ser.in_waiting)
                
                # 将字节数据转换为用空格分隔的大写十六进制字符串，例如: 54 2C 68 08 ...
                hex_data = binascii.hexlify(raw_data).decode('utf-8').upper()
                formatted_hex = ' '.join(hex_data[i:i+2] for i in range(0, len(hex_data), 2))
                
                print(formatted_hex, end=" ", flush=True)
                
            time.sleep(0.01) # 避免 CPU 占用过高

    except serial.SerialException as e:
        print(f"❌ 串口错误: {e}")
        print("请检查：1. COM3 是否被其他软件(如串口助手)占用；2. 线序 TX/RX 是否接反。")
    except KeyboardInterrupt:
        print("\n\n🛑 停止读取。")
    finally:
        if 'ser' in locals() and ser.is_open:
            ser.close()
            print("🔌 串口已关闭。")

if __name__ == "__main__":
    # 如果 115200 打印出来的都是 00 00 或者完全没有规律的乱码，
    # 可以尝试修改波特率为 230400, 128000 或 153600
    sniff_lidar_data("COM3", 115200)