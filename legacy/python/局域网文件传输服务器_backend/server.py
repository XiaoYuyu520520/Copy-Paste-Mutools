import os
import sys
import yaml
from pathlib import Path
from fastapi import FastAPI, HTTPException
from fastapi.responses import FileResponse, JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

# --- 1. 路径和配置处理 (打包核心逻辑) ---

def get_base_path():
    """获取资源的根路径，兼容开发环境和 PyInstaller 打包后的环境"""
    # 如果是被 PyInstaller 打包
    if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'):
        return sys._MEIPASS
    # 如果是正常的 .py 文件运行
    else:
        # 我们假设 frontend 文件夹在 backend 文件夹的上一级
        return os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

BASE_PATH = get_base_path()
print(f"资源根目录 (Base Path): {BASE_PATH}")

# 定义一个内置的默认配置
DEFAULT_CONFIG = {
    'server': {
        'host': '0.0.0.0',
        'port': 8000
    },
    'files': {
        # 默认共享 exe 文件旁边的 "shared_files" 文件夹
        'shared_directory': './shared_files'
    }
}

# 优先加载外部 config.yaml，如果不存在则使用内置的默认配置
config_path = Path("config.yaml")
if config_path.is_file():
    print("检测到外部 config.yaml，正在加载...")
    with open(config_path, "r", encoding="utf-8") as f:
        config = yaml.safe_load(f)
else:
    print("未检测到外部 config.yaml，使用内置默认配置。")
    config = DEFAULT_CONFIG

# 解析共享目录路径
# Path.resolve() 会将相对路径（如./shared_files）解析为基于当前工作目录的绝对路径
# 这意味着共享文件夹将永远是相对于你运行 .exe 的位置
SHARED_DIR = Path(config["files"]["shared_directory"]).resolve()
# 程序启动时就创建默认的共享文件夹，如果它不存在的话
if not SHARED_DIR.is_dir():
    print(f"共享目录 '{SHARED_DIR}' 不存在，正在自动创建...")
    SHARED_DIR.mkdir(parents=True, exist_ok=True)


# --- 2. FastAPI 应用初始化 ---
app = FastAPI()

# 中间件配置
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# --- 3. API 路由 (与之前版本相同) ---
@app.get("/api/files")
@app.get("/api/files/{sub_path:path}")
async def list_files(sub_path: str = ""):
    try:
        current_path = SHARED_DIR.joinpath(sub_path).resolve()
        if not current_path.exists() or not current_path.is_dir() or SHARED_DIR not in current_path.parents and current_path != SHARED_DIR:
             raise HTTPException(status_code=403, detail="禁止访问或目录不存在")
        
        items = []
        sorted_dir = sorted(current_path.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower()))
        for item_path in sorted_dir:
            relative_item_path = item_path.relative_to(SHARED_DIR)
            items.append({
                "name": item_path.name,
                "path": str(relative_item_path).replace("\\", "/"),
                "is_directory": item_path.is_dir(),
            })
        return JSONResponse(content=items)
    except Exception as e:
        print(f"!!! An error occurred in list_files: {e}") 
        raise HTTPException(status_code=500, detail=f"服务器内部错误: {e}")

@app.get("/api/download/{file_path:path}")
async def download_file(file_path: str):
    try:
        full_path = SHARED_DIR.joinpath(file_path).resolve()
        if not full_path.is_file() or SHARED_DIR not in full_path.parents:
            raise HTTPException(status_code=403, detail="禁止访问或文件不存在")
        return FileResponse(path=full_path, media_type='application/octet-stream', filename=full_path.name)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"服务器内部错误: {e}")

# --- 4. 托管前端文件 (路径已修改为动态) ---
# PyInstaller 会将 frontend 文件夹打包到 BASE_PATH
frontend_path = os.path.join(BASE_PATH, "frontend")
print(f"前端文件目录 (Frontend Path): {frontend_path}")
app.mount("/", StaticFiles(directory=frontend_path, html=True), name="static")

# --- 5. 主程序入口 (与之前版本相同) ---
if __name__ == "__main__":
    import uvicorn
    server_config = config.get("server", {})
    host = server_config.get("host", "0.0.0.0")
    port = server_config.get("port", 8000)
    
    print(f"服务器将在 http://{host}:{port} 启动")
    print(f"请确保在防火墙中开放 TCP 端口 {port}")
    print(f"当前共享目录为: {SHARED_DIR}")
    
    uvicorn.run(app, host=host, port=port)