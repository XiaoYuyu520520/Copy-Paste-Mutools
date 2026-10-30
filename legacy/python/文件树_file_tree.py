import tkinter as tk
from tkinter import ttk, filedialog
import os
import json

CONFIG_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "config.json")

DEFAULT_IGNORE_DIRS = {
    "node_modules", ".git", "__pycache__", ".venv", "venv",
    ".next", "dist", "build", ".cache", ".idea", ".vscode",
    ".svn", ".hg", "target", "bin", "obj", "vendor", ".tox",
    ".eggs", "egg-info", ".mypy_cache", ".pytest_cache",
}


class FileTreeApp:
    def __init__(self, root):
        self.root = root
        root.title("文件树查看器 (tree 风格)")
        root.geometry("900x700")

        self.last_path = self.load_config()

        top_frame = ttk.Frame(root)
        top_frame.pack(fill=tk.X, padx=5, pady=5)

        self.path_var = tk.StringVar(value=self.last_path or "")
        path_entry = ttk.Entry(top_frame, textvariable=self.path_var)
        path_entry.pack(side=tk.LEFT, fill=tk.X, expand=True, padx=(0, 5))

        browse_btn = ttk.Button(top_frame, text="选择文件夹", command=self.select_folder)
        browse_btn.pack(side=tk.RIGHT)

        btn_frame = ttk.Frame(root)
        btn_frame.pack(side=tk.RIGHT, fill=tk.Y, padx=(0, 5), pady=(40, 5))

        copy_btn = tk.Button(btn_frame, text="一键复制", command=self.copy_tree,
                             bg="#3498db", fg="white", relief=tk.RAISED,
                             padx=15, pady=6, cursor="hand2", font=("", 11))
        copy_btn.pack(side=tk.TOP, pady=(0, 10))

        copy_close_btn = tk.Button(btn_frame, text="复制并关闭", command=self.copy_and_close,
                                   bg="#e74c3c", fg="white", relief=tk.RAISED,
                                   padx=15, pady=6, cursor="hand2", font=("", 11))
        copy_close_btn.pack(side=tk.TOP)

        text_frame = ttk.Frame(root)
        text_frame.pack(fill=tk.BOTH, expand=True, padx=5, pady=(0, 5))

        scrollbar = ttk.Scrollbar(text_frame, orient=tk.VERTICAL)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)

        self.text = tk.Text(text_frame, wrap=tk.NONE, font=("Consolas", 10), yscrollcommand=scrollbar.set)
        self.text.pack(fill=tk.BOTH, expand=True)
        scrollbar.config(command=self.text.yview)

        h_scrollbar = ttk.Scrollbar(text_frame, orient=tk.HORIZONTAL, command=self.text.xview)
        h_scrollbar.pack(side=tk.BOTTOM, fill=tk.X)
        self.text.config(xscrollcommand=h_scrollbar.set)

        self.text.config(state=tk.DISABLED)

        if self.last_path and os.path.isdir(self.last_path):
            self.generate_tree(self.last_path)

    def load_config(self):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                data = json.load(f)
                return data.get("last_path", "")
        except (FileNotFoundError, json.JSONDecodeError):
            return ""

    def save_config(self, path):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                data = json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            data = {}
        try:
            data["last_path"] = path
            with open(CONFIG_FILE, "w", encoding="utf-8") as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
        except Exception:
            pass

    def load_ignore_dirs(self):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                data = json.load(f)
                return set(data.get("ignore_dirs", []))
        except (FileNotFoundError, json.JSONDecodeError):
            return set()

    def select_folder(self):
        folder = filedialog.askdirectory()
        if folder:
            self.path_var.set(folder)
            self.save_config(folder)
            self.generate_tree(folder)

    def generate_tree(self, root_path):
        self.ignore_dirs = self.load_ignore_dirs() or DEFAULT_IGNORE_DIRS
        self.text.config(state=tk.NORMAL)
        self.text.delete("1.0", tk.END)
        divider = "─" * 50
        self.text.insert(tk.END, f"{root_path}\n{divider}\n")
        self._write_tree(root_path, "", "")
        self.text.config(state=tk.DISABLED)

    def _write_tree(self, path, prefix, child_prefix):
        try:
            entries = sorted(os.listdir(path), key=lambda x: (not os.path.isdir(os.path.join(path, x)), x.lower()))
        except PermissionError:
            return

        filtered = []
        for entry in entries:
            full_path = os.path.join(path, entry)
            is_dir = os.path.isdir(full_path)
            if is_dir and entry in self.ignore_dirs:
                continue
            filtered.append((entry, is_dir, full_path))

        for i, (entry, is_dir, full_path) in enumerate(filtered):
            is_last = i == len(filtered) - 1

            connector = "└── " if is_last else "├── "
            self.text.insert(tk.END, f"{prefix}{connector}{entry}{'/' if is_dir else ''}\n")

            if is_dir:
                next_prefix = child_prefix + ("    " if is_last else "│   ")
                self._write_tree(full_path, child_prefix + ("    " if is_last else "│   "), next_prefix)

    def get_tree_text(self):
        return self.text.get("1.0", tk.END).strip()

    def copy_tree(self):
        text = self.get_tree_text()
        if text:
            self.root.clipboard_clear()
            self.root.clipboard_append(text)

    def copy_and_close(self):
        self.copy_tree()
        self.root.destroy()


if __name__ == "__main__":
    root = tk.Tk()
    app = FileTreeApp(root)
    root.mainloop()
