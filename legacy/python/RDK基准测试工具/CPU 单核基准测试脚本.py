import time
import math
import os
import platform
import multiprocessing

def get_system_info():
    """获取基础系统信息"""
    try:
        uname = platform.uname()
        info = f"{uname.system} {uname.release} ({uname.machine})"
        cores = multiprocessing.cpu_count()
        return info, cores
    except:
        return "Unknown System", 0

def benchmark_integer(limit=25000):
    """
    整数性能测试：查找指定范围内的质数
    涉及大量整数除法、取模、比较
    """
    start_time = time.time()
    count = 0
    # 简单的试除法，不进行过度优化，单纯为了消耗CPU周期
    for num in range(2, limit):
        is_prime = True
        # 只需检查到 sqrt(num)
        for i in range(2, int(num ** 0.5) + 1):
            if num % i == 0:
                is_prime = False
                break
        if is_prime:
            count += 1
    end_time = time.time()
    return end_time - start_time

def benchmark_float(iterations=2000000):
    """
    浮点性能测试：执行高强度的数学函数运算
    涉及 sin, cos, sqrt, 浮点加乘
    """
    start_time = time.time()
    val = 0.0
    # 模拟复杂的科学计算负载
    for i in range(1, iterations):
        # 混合运算，强制调用 FPU
        val += math.sin(i) * math.cos(i) + math.sqrt(i)
    end_time = time.time()
    return end_time - start_time

def calculate_score(duration, baseline_time):
    """
    计算分数
    公式：(基准时间 / 实际时间) * 1000
    """
    if duration == 0: return 0
    return int((baseline_time / duration) * 1000)

def main():
    # --- 配置参数 ---
    # 调整这些参数可以改变测试时长
    INT_LIMIT = 50000        # 整数测试范围
    FLOAT_ITERS = 3000000    # 浮点运算次数
    
    # --- 基准线定义 ---
    # 假设某台标准 PC (如 i5-8250U) 完成该任务分别需要 1.0 秒
    # 这样如果耗时 1.0 秒，得分就是 1000 分
    # 你可以把这个看作是 "Geekbench-Like" 的相对分数
    BASELINE_INT = 1.25   
    BASELINE_FLOAT = 1.15 

    sys_info, cores = get_system_info()
    
    print(f"=== 🧠 CPU 单核性能基准测试 (Python) ===")
    print(f"系统信息: {sys_info}")
    print(f"逻辑核心: {cores} 核")
    print(f"解释器  : Python {platform.python_version()}")
    print("---------------------------------------")

    print("🔥 正在预热 CPU (唤醒高性能核心)...")
    # 简单跑一下，让 CPU 从休眠状态唤醒，提升频率
    benchmark_integer(10000)
    
    print("---------------------------------------")
    print(f"1️⃣  开始整数 (Integer) 测试...")
    print(f"    负载: 查找 {INT_LIMIT} 以内的质数")
    t_int = benchmark_integer(INT_LIMIT)
    score_int = calculate_score(t_int, BASELINE_INT)
    print(f"    ✅ 耗时: {t_int:.4f} 秒")
    print(f"    🏆 整数得分: {score_int}")

    print("---------------------------------------")
    print(f"2️⃣  开始浮点 (Float) 测试...")
    print(f"    负载: {FLOAT_ITERS} 次混合三角函数运算")
    t_float = benchmark_float(FLOAT_ITERS)
    score_float = calculate_score(t_float, BASELINE_FLOAT)
    print(f"    ✅ 耗时: {t_float:.4f} 秒")
    print(f"    🏆 浮点得分: {score_float}")

    print("=======================================")
    print(f"🌟 综合单核分数: {int((score_int + score_float) / 2)}")
    print("=======================================")

if __name__ == "__main__":
    main()