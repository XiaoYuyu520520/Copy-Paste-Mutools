import os
import time
import subprocess
import sys

def convert_size(size_bytes):
    """转换字节为人类易读格式"""
    if size_bytes == 0:
        return "0 B"
    size_name = ("B", "KB", "MB", "GB", "TB")
    i = int(os.math.floor(os.math.log(size_bytes, 1024)))
    p = os.math.pow(1024, i)
    s = round(size_bytes / p, 2)
    return f"{s} {size_name[i]}"

def drop_caches():
    """
    【关键优化】清除 Linux 系统页缓存
    需要 sudo 权限，否则读取测试测的其实是内存速度
    """
    print("⚡ 正在清除系统缓存以确保读取真实性 (需要 sudo)...")
    try:
        # 方式1: 尝试直接用 shell 命令
        subprocess.run("sync", shell=True)
        subprocess.run("echo 3 > /proc/sys/vm/drop_caches", shell=True)
    except Exception:
        pass
    
    # 方式2: 如果上面失败，尝试 sudo 提权方式
    try:
        subprocess.run(["sudo", "sh", "-c", "echo 3 > /proc/sys/vm/drop_caches"], check=True)
        print("✅ 缓存清除成功")
    except subprocess.CalledProcessError:
        print("⚠️ 警告: 缓存清除失败！读取速度可能虚高 (即内存速度)")
        print("   请尝试使用 'sudo python3 xxx.py' 运行此脚本")

def test_disk_io(file_path, size_gb=1):
    file_size_bytes = int(size_gb * 1024 * 1024 * 1024)
    # 使用 64MB 作为块大小，平衡内存占用和系统调用开销
    block_size = 64 * 1024 * 1024 
    blocks = file_size_bytes // block_size
    remaining = file_size_bytes % block_size

    print(f"=== 🚀 磁盘 I/O 性能测试 ===")
    print(f"文件路径: {file_path}")
    print(f"测试大小: {size_gb} GB")
    print("---------------------------------")

    # --- 1. 写入测试 ---
    print("📝 正在进行写入测试...")
    
    # 【优化】: 预先生成随机数据，避免循环中 CPU 生成数据成为瓶颈
    random_data = os.urandom(block_size)
    if remaining > 0:
        random_remainder = random_data[:remaining]

    start_write = time.time()
    try:
        with open(file_path, 'wb') as f:
            for _ in range(blocks):
                f.write(random_data)
            if remaining > 0:
                f.write(random_remainder)
            
            # 【关键】: 强制将缓冲区数据刷入物理磁盘
            f.flush()
            os.fsync(f.fileno())
    except Exception as e:
        print(f"❌ 写入失败: {e}")
        return

    end_write = time.time()
    write_time = end_write - start_write
    write_speed = file_size_bytes / write_time
    print(f"✅ 写入完成: {write_time:.2f} 秒")
    print(f"🔥 写入速度: {convert_size(write_speed)}/s")
    print("---------------------------------")

    # --- 2. 清除缓存 ---
    drop_caches()
    print("---------------------------------")

    # --- 3. 读取测试 ---
    print("📖 正在进行读取测试...")
    start_read = time.time()
    try:
        with open(file_path, 'rb') as f:
            while True:
                # 这里不需要处理数据，只负责搬运
                data = f.read(block_size)
                if not data:
                    break
    except Exception as e:
        print(f"❌ 读取失败: {e}")
        return

    end_read = time.time()
    read_time = end_read - start_read
    read_speed = file_size_bytes / read_time
    print(f"✅ 读取完成: {read_time:.2f} 秒")
    print(f"🔥 读取速度: {convert_size(read_speed)}/s")
    print("=================================")

    # 清理文件
    if os.path.exists(file_path):
        os.remove(file_path)

if __name__ == "__main__":
    # 确保 math 库被导入 (上面代码用了 os.math 是为了兼容性，也可以直接 import math)
    import math
    os.math = math
    
    # 默认在当前目录测试
    target_file = os.path.join(os.getcwd(), "disk_bench.tmp")
    
    # 检查是否以 sudo 运行（为了清除缓存）
    if os.geteuid() != 0:
        print("💡 建议: 使用 'sudo python3 ...' 运行以获得最准确的读取速度")
        print("   (否则无法清除 Page Cache，读取速度会显示为内存速度)")
        print("")

    test_disk_io(target_file, size_gb=1)