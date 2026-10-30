import os
import sys
import signal
from pathlib import Path

# ===================== 可自定义的过滤配置 =====================
# 需要忽略的文件夹名称（支持完整名称或通配符）
IGNORED_DIRS = {
    # 虚拟环境相关
    "venv", ".venv", "env", ".env", "virtualenv", ".virtualenv",
    # 缓存相关
    "__pycache__", ".cache", "cache", ".pytest_cache",
    # 前端/依赖相关
    "node_modules", "bower_components",
    # 版本控制/构建相关
    ".git", ".svn", "dist", "build", "target", "out",
    # 系统/日志相关
    ".logs", "logs", ".tmp", "tmp", ".DS_Store"
}

# 最大递归深度（避免层级过深）
MAX_DEPTH = 5
# 单个目录下最多展示的子目录数（超出用...表示）
MAX_SUBDIRS = 8
# ============================================================

def signal_handler(sig, frame):
    """处理Ctrl+C退出信号，优雅结束程序"""
    print("\n\n✨ 程序已通过Ctrl+C优雅退出")
    sys.exit(0)

# 注册Ctrl+C信号处理函数
signal.signal(signal.SIGINT, signal_handler)

def is_ignored_dir(dir_name: str) -> bool:
    """判断目录是否属于需要忽略的类型"""
    # 精确匹配忽略列表中的名称
    if dir_name in IGNORED_DIRS:
        return True
    # 额外匹配虚拟环境特征（如以venv-开头、.venv-开头）
    if dir_name.startswith(("venv-", ".venv-", "env-", ".env-")):
        return True
    return False

def collect_valid_dirs(root_dir: Path, recursive: bool = True, max_depth: int = MAX_DEPTH) -> list:
    """
    收集所有非忽略目录，限制递归深度
    :param root_dir: 根目录
    :param recursive: 是否递归扫描
    :param max_depth: 最大递归深度
    :return: 元组列表 (目录路径, 深度)
    """
    valid_dirs = []
    
    def _recursive_collect(current_dir: Path, current_depth: int):
        # 超过最大深度则停止递归
        if current_depth > max_depth:
            return
        
        # 收集当前目录（只要不是根目录且未被忽略）
        if current_dir != root_dir and not is_ignored_dir(current_dir.name):
            valid_dirs.append((current_dir, current_depth))
        
        # 递归扫描子目录（跳过忽略的目录）
        if recursive:
            subdirs = []
            for item in current_dir.iterdir():
                if item.is_dir() and not is_ignored_dir(item.name):
                    subdirs.append(item)
            
            # 子目录数量超过阈值时，只展示前N个，其余用...表示
            if len(subdirs) > MAX_SUBDIRS:
                for subdir in subdirs[:MAX_SUBDIRS]:
                    _recursive_collect(subdir, current_depth + 1)
                valid_dirs.append((f"{current_dir}/...", current_depth + 1))
            else:
                for subdir in subdirs:
                    _recursive_collect(subdir, current_depth + 1)
    
    # 先添加根目录
    valid_dirs.append((root_dir, 0))
    # 递归收集子目录
    _recursive_collect(root_dir, 0)
    return valid_dirs

def print_tree(dirs_with_depth: list, root_dir: Path):
    """以简洁的tree风格打印目录结构"""
    # 按深度和路径排序
    dirs_with_depth.sort(key=lambda x: (x[1], str(x[0])))
    
    # 记录每个深度的最后一个元素
    last_item_per_depth = {}
    total = len(dirs_with_depth)
    for idx, (dir_path, depth) in enumerate(dirs_with_depth):
        # 判断是否是当前深度的最后一项
        is_last = True
        for i in range(idx + 1, total):
            if dirs_with_depth[i][1] == depth:
                is_last = False
                break
        last_item_per_depth[depth] = is_last
    
    for dir_path, depth in dirs_with_depth:
        # 构建层级缩进
        indent = ""
        for level in range(depth):
            if last_item_per_depth.get(level, False):
                indent += "    "  # 上一级是最后一项，用空格
            else:
                indent += "│   "  # 上一级不是最后一项，用竖线
        
        # 处理...标记
        if isinstance(dir_path, str) and dir_path.endswith("/..."):
            dir_name = "..."
        else:
            dir_name = dir_path.name if isinstance(dir_path, Path) else dir_path
        
        # 打印目录项（最后一项用└──，其余用├──）
        if last_item_per_depth[depth]:
            print(f"{indent}└── {dir_name}")
        else:
            print(f"{indent}├── {dir_name}")

def normalize_input_path(input_str: str) -> Path:
    """标准化输入路径：解析~、文件转目录、绝对路径"""
    expanded_path = os.path.expanduser(input_str.strip())
    path_obj = Path(expanded_path)
    
    if path_obj.is_file():
        dir_path = path_obj.parent
        print(f"ℹ️  检测到输入是文件路径，自动切换为目录：{dir_path}")
    else:
        dir_path = path_obj
    
    return dir_path.absolute()

def main():
    print("="*60)
    print("🎯 目录树过滤工具（忽略虚拟环境/缓存等文件夹）")
    print(f"⚠️  忽略的文件夹类型：{', '.join(sorted(IGNORED_DIRS))}")
    print("💡 使用说明：输入路径（支持~），按回车执行；按Ctrl+C退出")
    print("="*60)
    
    while True:
        try:
            user_input = input("\n请输入目标路径（按Ctrl+C退出）：")
            
            # 空输入默认当前目录
            if not user_input:
                target_dir = Path.cwd()
                print(f"ℹ️  未输入路径，使用当前目录：{target_dir}")
            else:
                target_dir = normalize_input_path(user_input)
            
            # 验证目录是否存在
            if not target_dir.is_dir():
                print(f"❌ 错误：目录 {target_dir} 不存在！")
                continue
            
            # 收集符合条件的目录
            print(f"\n🔍 正在扫描目录：{target_dir}（忽略虚拟环境/缓存等文件夹）...")
            valid_dirs = collect_valid_dirs(target_dir)
            
            # 打印结果
            if len(valid_dirs) <= 1:  # 只有根目录
                print("📭 除根目录外，无其他可展示的目录（已过滤所有忽略项）")
            else:
                print("\n✅ 过滤后的目录结构：")
                print_tree(valid_dirs, target_dir)
        
        except Exception as e:
            print(f"❌ 程序出错：{str(e)}，请重试")

if __name__ == "__main__":
    main()
