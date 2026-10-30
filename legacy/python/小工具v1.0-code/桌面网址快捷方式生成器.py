import tkinter as tk
from tkinter import filedialog, messagebox
import os
import re

class App:
    def __init__(self, root):
        self.root = root
        self.root.title("网址快捷方式生成器")
        self.root.geometry("450x220")
        self.root.resizable(False, False)

        main_frame = tk.Frame(root, padx=15, pady=15)
        main_frame.pack(fill="both", expand=True)

        # --- 快捷方式名称 ---
        tk.Label(main_frame, text="快捷方式名称:").grid(row=0, column=0, sticky="w", pady=2)
        self.name_entry = tk.Entry(main_frame, width=40)
        self.name_entry.grid(row=0, column=1, columnspan=2, sticky="ew")

        # --- 网址 ---
        tk.Label(main_frame, text="目标网址:").grid(row=1, column=0, sticky="w", pady=2)
        self.url_entry = tk.Entry(main_frame, width=40)
        self.url_entry.grid(row=1, column=1, columnspan=2, sticky="ew")
        self.url_entry.insert(0, "https://")

        # --- 保存位置 ---
        tk.Label(main_frame, text="保存位置:").grid(row=2, column=0, sticky="w", pady=2)
        self.path_entry = tk.Entry(main_frame, width=30)
        self.path_entry.grid(row=2, column=1, sticky="ew")
        # 自动获取桌面路径
        desktop_path = os.path.join(os.path.expanduser("~"), "Desktop")
        self.path_entry.insert(0, desktop_path)
        
        browse_button = tk.Button(main_frame, text="浏览...", command=self.browse_folder)
        browse_button.grid(row=2, column=2, sticky="ew", padx=(5, 0))

        # --- 创建按钮 ---
        create_button = tk.Button(main_frame, text="创建快捷方式", command=self.create_shortcut, bg="#4CAF50", fg="white", font=("Arial", 10, "bold"))
        create_button.grid(row=3, column=0, columnspan=3, sticky="ew", pady=(15, 5), ipady=5)

        # --- 状态栏 ---
        self.status_label = tk.Label(main_frame, text="准备就绪", fg="grey")
        self.status_label.grid(row=4, column=0, columnspan=3, sticky="w", pady=(5, 0))
        
    def browse_folder(self):
        """弹出文件夹选择对话框"""
        folder_selected = filedialog.askdirectory()
        if folder_selected:
            self.path_entry.delete(0, tk.END)
            self.path_entry.insert(0, folder_selected)

    def create_shortcut(self):
        """核心功能：创建.bat文件"""
        name = self.name_entry.get().strip()
        url = self.url_entry.get().strip()
        save_path = self.path_entry.get().strip()

        # --- 输入验证 ---
        if not name or not url or not save_path:
            messagebox.showerror("错误", "所有字段都不能为空！")
            return
            
        if not url.startswith("http://") and not url.startswith("https://"):
            messagebox.showerror("错误", "请输入一个有效的网址 (以 http:// 或 https:// 开头)。")
            return

        if not os.path.isdir(save_path):
            messagebox.showerror("错误", "指定的保存位置不是一个有效的文件夹。")
            return

        # --- 清理文件名中的非法字符 ---
        safe_name = re.sub(r'[\\/*?:"<>|]', "", name)
        file_path = os.path.join(save_path, f"{safe_name}.bat")

        # --- 写入 .bat 文件内容 ---
        # @echo off: 运行时不显示命令本身
        # start "" "URL": 使用start命令, ""是为窗口指定一个空标题，防止URL中含空格时出错
        bat_content = f'@echo off\nstart "" "{url}"'

        try:
            with open(file_path, "w", encoding="utf-8") as f:
                f.write(bat_content)
            self.status_label.config(text=f"成功！快捷方式已创建在: {file_path}", fg="green")
            messagebox.showinfo("成功", f"快捷方式 '{safe_name}.bat' 已成功创建！")
        except Exception as e:
            self.status_label.config(text=f"失败: {e}", fg="red")
            messagebox.showerror("写入失败", f"创建文件时发生错误: \n{e}")

if __name__ == "__main__":
    root = tk.Tk()
    app = App(root)
    root.mainloop()
