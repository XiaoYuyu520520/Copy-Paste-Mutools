# backend/server_3.py
import os
import sys
import yaml
import zipfile
import io
from pathlib import Path
from typing import List
from pydantic import BaseModel
from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
# 修改此行
from fastapi.responses import StreamingResponse, FileResponse

# --- 1. 配置加载 (与之前版本相同) ---
# ... (从 server_2.py 完整复制这部分代码)
def get_base_path():
    if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'): return sys._MEIPASS
    else: return os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BASE_PATH = get_base_path()
DEFAULT_CONFIG = {'server': {'host': '0.0.0.0', 'port': 8000},'files': {'shared_directory': './shared_files'}}
config_path = Path("config.yaml")
if config_path.is_file():
    with open(config_path, "r", encoding="utf-8") as f: config = yaml.safe_load(f)
else: config = DEFAULT_CONFIG
SHARED_DIR = Path(config["files"]["shared_directory"]).resolve()
if not SHARED_DIR.is_dir(): SHARED_DIR.mkdir(parents=True, exist_ok=True)
# ... (复制结束)

app = FastAPI()
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_credentials=True, allow_methods=["*"], allow_headers=["*"])

# --- 2. API 路由 (部分复用，新增批量下载) ---

# 定义批量下载请求的数据模型
class DownloadRequest(BaseModel):
    paths: List[str]

# list_files 路由保持不变...
@app.get("/api/files/{sub_path:path}", tags=["files"])
@app.get("/api/files", tags=["files"])
async def list_files(sub_path: str = ""):
    # ... (从 server_2.py 完整复制 list_files 函数)
    try:
        current_path = SHARED_DIR.joinpath(sub_path).resolve()
        if not current_path.exists() or not current_path.is_dir() or SHARED_DIR not in current_path.parents and current_path != SHARED_DIR: raise HTTPException(status_code=403)
        items = []
        for item_path in sorted(current_path.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower())):
            items.append({"name": item_path.name, "path": str(item_path.relative_to(SHARED_DIR)).replace("\\", "/"),"is_directory": item_path.is_dir()})
        return items
    except Exception as e: raise HTTPException(status_code=500, detail=f"Server error: {e}")
    # ... (复制结束)


# ✨✨✨ --- 全新的批量下载路由 --- ✨✨✨
@app.post("/api/download/batch", tags=["download"])
async def download_batch(request: DownloadRequest):
    """
    接收一个包含文件和文件夹路径的列表，将它们全部打包成一个 zip 文件。
    """
    zip_buffer = io.BytesIO()
    with zipfile.ZipFile(zip_buffer, "w", zipfile.ZIP_DEFLATED) as zf:
        for item_path_str in request.paths:
            # 安全检查
            full_path = SHARED_DIR.joinpath(item_path_str).resolve()
            if SHARED_DIR not in full_path.parents and full_path != SHARED_DIR:
                continue # 如果路径无效或不安全，则跳过

            if full_path.is_file():
                # 如果是文件，直接写入
                archive_path = full_path.relative_to(SHARED_DIR.parent) # 使用上一级目录，以保留根文件夹名
                zf.write(full_path, archive_path)
            elif full_path.is_dir():
                # 如果是文件夹，遍历并写入
                for root, _, files in os.walk(full_path):
                    for file in files:
                        file_abs_path = Path(root) / file
                        archive_path = file_abs_path.relative_to(SHARED_DIR.parent)
                        zf.write(file_abs_path, archive_path)
    
    zip_buffer.seek(0)
    return StreamingResponse(
        zip_buffer,
        media_type="application/zip",
        headers={"Content-Disposition": f"attachment; filename=\"shared_files.zip\""}
    )

# --- 3. 托管前端 ---
# --- 3. 托管前端 (永久修复版) ---
frontend_path = os.path.join(BASE_PATH, "frontend")

# ✨ 新增：我们明确地为根路径 "/" 指定返回 index_3.html
# 这样可以确保无论缓存如何，用户访问根目录时总能得到最新的主页。
@app.get("/", include_in_schema=False)
async def read_index():
    return FileResponse(os.path.join(frontend_path, 'index_3.html'))

# 修改：现在这个挂载点只负责处理 CSS, JS 等静态文件，因为 "/" 路径已经被上面的函数处理了。
app.mount("/", StaticFiles(directory=frontend_path), name="static")






class SinglePageApplication(StaticFiles):
    async def get_response(self, path: str, scope):
        try:
            return await super().get_response(path, scope)
        except HTTPException as ex:
            if ex.status_code == 404:
                return await super().get_response("index_3.html", scope)
            raise ex
app.mount("/", SinglePageApplication(directory=frontend_path, html=True), name="static")

# --- 4. 启动 ---
if __name__ == "__main__":
    import uvicorn
    server_config = config.get("server", {})
    host, port = server_config.get("host", "0.0.0.0"), server_config.get("port", 8000)
    
    print(f"服务器将在 http://{host}:{port} 启动 (v3 - 已修复缓存问题)")
    print(f"当前共享目录为: {SHARED_DIR}")
    
    # 提示：在开发时，可以暂时关闭 uvicorn 的 reload 功能以避免缓存混淆
    uvicorn.run("server_3:app", host=host, port=port)