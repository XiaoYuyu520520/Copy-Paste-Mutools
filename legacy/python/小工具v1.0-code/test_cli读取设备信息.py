import subprocess
import json
import os

def run_test(exe_path, args=None):
    """
    运行测试的核心函数。
    - exe_path: GetDeviceInfo.exe 的路径。
    - args: 一个包含命令行参数的列表，例如 ['--json', 'cpu']。
    """
    if not os.path.exists(exe_path):
        print(f"错误：在路径 '{exe_path}' 下找不到 GetDeviceInfo.exe。")
        print("请确保此脚本与 dist 文件夹在同一目录下，并且你已经成功打包了程序。")
        return

    command = [exe_path]
    if args:
        command.extend(args)
        
    print("="*60)
    print(f"🚀 正在执行命令: {' '.join(command)}")
    print("="*60)

    try:
        # ### 核心修正 ###
        # 准备传递给 subprocess.run 的参数字典
        run_kwargs = {
            "capture_output": True, 
            "text": True, 
            "encoding": 'utf-8', 
            "timeout": 15
        }
        
        # 如果是交互模式 (args is None)，则自动提供一个“回车”作为输入
        if args is None:
            run_kwargs['input'] = '\n'

        # 运行子进程
        result = subprocess.run(command, **run_kwargs)

        if result.returncode == 0:
            if args and "--json" in args:
                try:
                    json_data = json.loads(result.stdout)
                    print("✅ CLI测试成功！成功解析返回的 JSON 数据：")
                    print(json.dumps(json_data, indent=4, ensure_ascii=False))
                except json.JSONDecodeError:
                    print("❌ CLI测试失败！命令有输出，但无法解析为 JSON。")
                    print("--- 原始输出 ---")
                    print(result.stdout)
            else:
                print("✅ 交互模式测试成功！程序已自动接收回车并正常退出。")
                print("--- 程序输出 (前5行) ---")
                for line in result.stdout.splitlines()[:5]:
                    print(line)
                print("...")
        else:
            print(f"❌ 命令执行失败！返回码: {result.returncode}")
            print("--- 错误输出 (stderr) ---")
            print(result.stderr)

    except subprocess.TimeoutExpired:
        # 现在，如果真的发生超时，这个提示就能正常显示了
        print("❌ 测试超时！命令执行超过15秒未返回。")
    except Exception as e:
        print(f"❌ 执行时发生未知错误: {e}")
    
    print("\n")


if __name__ == "__main__":
    exe_file_path = os.path.join("dist", "GetDeviceInfo.exe")

    test_cases = [
        None,
        ["--json", "system"],
        ["--json", "cpu"],
        ["--json", "gpu"],
        ["--json", "memory"],
        ["--json", "disk"],
        ["--json", "all"],
        ["--json", "invalid_param"]
    ]

    print("### 开始测试 GetDeviceInfo.exe 的 CLI 功能 ###\n")
    for case in test_cases:
        run_test(exe_file_path, case)
    
    print("### 所有测试执行完毕 ###")