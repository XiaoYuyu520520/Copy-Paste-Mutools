# backend/server_for_packaging.py (Definitive Final Version)

import os
import sys
import yaml
import zipfile
import io
from pathlib import Path
from typing import List
from pydantic import BaseModel
import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse, FileResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

# --- 1. PATH AND CONFIGURATION LOGIC FOR PACKAGING ---

def get_bundle_dir():
    if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'):
        return sys._MEIPASS
    else:
        return os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

def get_exe_dir():
    if getattr(sys, 'frozen', False) and hasattr(sys, '_MEIPASS'):
        return os.path.dirname(sys.executable)
    else:
        return os.path.dirname(os.path.abspath(__file__))

BUNDLE_DIR = get_bundle_dir()
EXE_DIR = get_exe_dir()

DEFAULT_CONFIG = {
    'server': {'host': '0.0.0.0', 'port': 8000},
    'files': {'shared_directory': './shared_files'}
}

config_path = os.path.join(EXE_DIR, "config.yaml")
if os.path.isfile(config_path):
    print(f"Loading external config: {config_path}")
    with open(config_path, "r", encoding="utf-8") as f:
        config = yaml.safe_load(f)
else:
    print("Using internal default configuration.")
    config = DEFAULT_CONFIG

shared_path_str = config["files"]["shared_directory"]
if Path(shared_path_str).is_absolute():
    SHARED_DIR = Path(shared_path_str).resolve()
else:
    SHARED_DIR = Path(os.path.join(EXE_DIR, shared_path_str)).resolve()

if not SHARED_DIR.is_dir():
    print(f"Shared directory not found. Creating it at: {SHARED_DIR}")
    SHARED_DIR.mkdir(parents=True, exist_ok=True)

# --- 2. FASTAPI APP AND API ROUTES ---
app = FastAPI()
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_credentials=True, allow_methods=["*"], allow_headers=["*"])

class DownloadRequest(BaseModel):
    paths: List[str]

@app.get("/api/files/{sub_path:path}", tags=["files"])
@app.get("/api/files", tags=["files"])
async def list_files(sub_path: str = ""):
    try:
        current_path = SHARED_DIR.joinpath(sub_path).resolve()
        if not current_path.exists() or not current_path.is_dir() or SHARED_DIR not in current_path.parents and current_path != SHARED_DIR:
            raise HTTPException(status_code=403, detail="Access denied or directory not found")
        items = []
        for item_path in sorted(current_path.iterdir(), key=lambda p: (not p.is_dir(), p.name.lower())):
            items.append({
                "name": item_path.name,
                "path": str(item_path.relative_to(SHARED_DIR)).replace("\\", "/"),
                "is_directory": item_path.is_dir()
            })
        return items
    except Exception as e:
        print(f"Error in list_files: {e}")
        raise HTTPException(status_code=500, detail=f"Server error: {e}")

@app.post("/api/download/batch", tags=["download"])
async def download_batch(request: DownloadRequest):
    zip_buffer = io.BytesIO()
    with zipfile.ZipFile(zip_buffer, "w", zipfile.ZIP_DEFLATED) as zf:
        for item_path_str in request.paths:
            full_path = SHARED_DIR.joinpath(item_path_str).resolve()
            if SHARED_DIR not in full_path.parents and full_path != SHARED_DIR:
                continue
            if full_path.is_file():
                archive_path = full_path.relative_to(SHARED_DIR.parent)
                zf.write(full_path, archive_path)
            elif full_path.is_dir():
                for root, _, files in os.walk(full_path):
                    for file in files:
                        file_abs_path = Path(root) / file
                        archive_path = file_abs_path.relative_to(SHARED_DIR.parent)
                        zf.write(file_abs_path, archive_path)
    zip_buffer.seek(0)
    return StreamingResponse(zip_buffer, media_type="application/zip", headers={"Content-Disposition": f"attachment; filename=\"shared_files.zip\""})

# --- 3. SERVE FRONTEND ---
frontend_path = os.path.join(BUNDLE_DIR, "frontend")

@app.get("/", include_in_schema=False)
async def read_index():
    return FileResponse(os.path.join(frontend_path, 'index_3.html'))

app.mount("/", StaticFiles(directory=frontend_path), name="static")

# --- 4. APP LAUNCHER (WITH DEFINITIVE --noconsole FIX) ---
if __name__ == "__main__":
    server_config = config.get("server", {})
    host, port = server_config.get("host", "0.0.0.0"), server_config.get("port", 8000)
    
    # Start with the default log config
    log_config = uvicorn.config.LOGGING_CONFIG
    
    # If frozen (packaged by PyInstaller in --noconsole mode),
    # replace the default config with a simple, safe one.
    if getattr(sys, 'frozen', False):
        log_config = {
            "version": 1,
            "disable_existing_loggers": True,
            "formatters": {
                "default": {
                    "format": "%(asctime)s - %(levelname)s - %(message)s",
                }
            },
            "handlers": {
                "default": {
                    "formatter": "default",
                    "class": "logging.StreamHandler",
                    "stream": "ext://sys.stderr",
                },
            },
            "loggers": {
                "uvicorn": {"handlers": ["default"], "level": "INFO", "propagate": False},
                "uvicorn.error": {"handlers": ["default"], "level": "INFO", "propagate": False},
                "uvicorn.access": {"handlers": ["default"], "level": "INFO", "propagate": False},
            },
        }

    print(f"Server starting on http://{host}:{port} (Final Corrected Version)")
    print(f"Sharing directory: {SHARED_DIR}")
    
    # Run Uvicorn with the appropriate log config
    uvicorn.run(app, host=host, port=port, log_config=log_config)