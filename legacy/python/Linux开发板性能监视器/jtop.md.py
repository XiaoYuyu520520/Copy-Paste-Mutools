from jtop import jtop

# --- 配置参数 ---
FREQ_DIVISOR = 1000  # KHz -> MHz
MB_DIVISOR = 1024**2 # Bytes -> MB

def run_monitor():
    with jtop() as jetson:
        if jetson.ok():
            # 获取 GPU 原始字典
            gpu_data = jetson.gpu['gpu']
            
            # 1. 频率与负载
            usage = gpu_data['status']['load'] * 100
            cur_freq = gpu_data['freq']['cur'] / FREQ_DIVISOR
            max_freq = gpu_data['freq']['max'] / FREQ_DIVISOR
            
            # 2. 电压 (使用 get 防止部分型号不存在该键值)
            voltage = gpu_data.get('volt', {}).get('cur', 0)
            
            # 3. 硬件引擎 (修正为 .engine)
            # 这里的 get('avg', 0) 是指获取平均负载百分比
            nvdec_load = jetson.engine.get('NVDEC', {}).get('avg', 0)
            nvenc_load = jetson.engine.get('NVENC', {}).get('avg', 0)
            
            # 4. 内存信息
            ram = jetson.memory.get('RAM', {})
            ram_used = ram.get('used', 0) / MB_DIVISOR
            ram_total = ram.get('tot', 0) / MB_DIVISOR

            # --- 打印输出 ---
            print("\n" + "="*30)
            print(f"GPU Usage:     {usage:.1f}%")
            print(f"GPU Freq:      {cur_freq:.0f} / {max_freq:.0f} MHz")
            print(f"GPU Voltage:   {voltage} mV")
            print("-" * 30)
            print(f"NVDEC (Dec):   {nvdec_load}%")
            print(f"NVENC (Enc):   {nvenc_load}%")
            print("-" * 30)
            print(f"RAM Usage:     {ram_used:.1f} / {ram_total:.1f} MB")
            print("="*30 + "\n")

if __name__ == "__main__":
    run_monitor()