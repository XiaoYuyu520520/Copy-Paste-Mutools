好的，这正是一个将应用程序变得更加专业和易于分发的绝佳想法！

将所有依赖（如 `config.yaml` 和 `frontend` 文件夹）打包进一个 `.exe` 文件中，让用户只需双击即可运行，这是完全可以实现的。我们将使用一个名为 **PyInstaller** 的流行工具来完成这个任务。

为了实现这个目标，我们需要做两件事：

1.  **修改 `server.py` 代码**：让它能够从程序内部（而不是外部文件系统）加载默认配置和前端文件。
2.  **使用 PyInstaller 打包**：通过一个命令将 Python 脚本、所有依赖库和前端文件夹全部打包成一个独立的 `.exe` 文件。

-----

### 策略：内部默认，外部优先

一个最佳实践是：

  * **内置默认配置**：在 Python 代码中直接写入一个默认的配置。
  * **内置前端文件**：将 `frontend` 文件夹打包到 `.exe` 内部。
  * **优先加载外部文件**：程序启动时，首先检查 `.exe`旁边**是否存在**一个 `config.yaml` 文件。如果存在，就优先使用这个外部配置。如果不存在，才使用代码中内置的默认配置。

这样做的好处是，您既可以获得一个开箱即用的`.exe`，又保留了让高级用户通过放置一个 `config.yaml` 文件来自定义配置的灵活性。

-----

### 第一步：修改 `server.py`

请用下面这个**完整的新版本**替换您现有的 `backend/server.py` 代码。新版本中包含了所有必要的逻辑。

```python
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
```

**代码修改摘要：**

  * **内置默认配置**：在代码顶部定义了 `DEFAULT_CONFIG`。
  * **智能配置加载**：程序会先找外部的 `config.yaml`，找不到再用内置的。
  * **动态路径**：使用 `get_base_path()` 函数来判断当前是开发模式还是打包模式，从而正确找到前端文件的位置。
  * **自动创建文件夹**：如果默认的 `shared_files` 文件夹不存在，程序会自动创建它，提升用户体验。

-----

### 第二步：使用 PyInstaller 打包

1.  **安装 PyInstaller**:

      * 在您的虚拟环境中，运行：
        ```bash
        pip install pyinstaller
        ```

2.  **执行打包命令**:

      * **打开一个新的命令行窗口**，并确保您位于 `backend` 目录下。
      * 运行以下命令：
        ```bash
        pyinstaller --name lan-share --onefile --add-data="../frontend;frontend" server.py
        ```

**命令解释**:

  * `pyinstaller`: 启动打包工具。
  * `--name lan-share`: 指定生成的 `.exe` 文件的名字为 `lan-share.exe`。
  * `--onefile`: 将所有东西打包成 **一个** `.exe` 文件。
  * `--add-data="../frontend;frontend"`: 这是最关键的一步。
      * 它告诉 PyInstaller：“找到 `server.py` 上一级的 `frontend` 文件夹 (`../frontend`)，然后把它整个添加到 `.exe` 内部，并在内部依然命名为 `frontend`”。
      * `来源;目标` 的格式，在 Windows 上使用分号 `;`，在 macOS/Linux 上使用冒号 `:`。
  * `server.py`: 你的主程序文件。

**打包过程可能需要几分钟。**

-----

### 第三步：运行和分发

1.  **找到 `.exe` 文件**:

      * 打包完成后，在 `backend` 目录下会出现一个新的 `dist` 文件夹。
      * 您的 `lan-share.exe` 文件就在 `dist` 文件夹里。

2.  **如何使用**:

      * 将 `lan-share.exe` 文件**单独**复制到任何您想放的地方，比如桌面。
      * **直接双击运行 `lan-share.exe`**。它会使用内置的配置，并自动在旁边创建一个 `shared_files` 文件夹。您只需将要共享的文件拖入其中即可。
      * (可选) 如果您想自定义端口或共享目录，只需在 `lan-share.exe` 旁边创建一个 `config.yaml` 文件，程序下次启动时就会自动读取它。

现在，您拥有了一个无需安装、不带任何散乱文件、即点即用的文件共享服务器了！