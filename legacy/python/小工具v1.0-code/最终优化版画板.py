import tkinter as tk
from tkinter import colorchooser, simpledialog, messagebox
import os
import sys
import json
import traceback

def get_base_path():
    """获取应用程序的基础路径，兼容正常运行和PyInstaller打包后的情况。"""
    if getattr(sys, 'frozen', False):
        return os.path.dirname(sys.executable)
    else:
        return os.path.dirname(os.path.abspath(__file__))

CONFIG_FILE = os.path.join(get_base_path(), "drawing_board_config.json")

class DrawingApp:
    def __init__(self, root):
        self.root = root
        self.root.title("简易画板")

        self.brush_color = "blue"
        self.brush_size = 3
        self.last_x, self.last_y = None, None

        self.canvas = tk.Canvas(root, bg="white", cursor="cross")
        self.canvas.pack(fill="both", expand=True)

        control_frame = tk.Frame(root)
        control_frame.pack(fill="x", pady=5)

        clear_button = tk.Button(control_frame, text="清空画布", command=self.clear_canvas)
        clear_button.pack(side="left", padx=5, ipadx=5, ipady=2)

        color_button = tk.Button(control_frame, text="选择颜色", command=self.choose_color)
        color_button.pack(side="left", padx=5, ipadx=5, ipady=2)
        
        size_button = tk.Button(control_frame, text="调整画笔粗细", command=self.choose_size)
        size_button.pack(side="left", padx=5, ipadx=5, ipady=2)
        
        self.canvas.bind("<B1-Motion>", self.draw)
        self.canvas.bind("<ButtonRelease-1>", self.reset_position)

    def draw(self, event):
        if self.last_x is None or self.last_y is None:
            self.last_x, self.last_y = event.x, event.y
            return
        self.canvas.create_line(
            (self.last_x, self.last_y, event.x, event.y),
            fill=self.brush_color, width=self.brush_size,
            capstyle=tk.ROUND, smooth=tk.TRUE
        )
        self.last_x, self.last_y = event.x, event.y

    def reset_position(self, event):
        self.last_x, self.last_y = None, None
    def clear_canvas(self): self.canvas.delete("all")
    def choose_color(self):
        color_code = colorchooser.askcolor(title="选择一个颜色")
        if color_code and color_code[1]: self.brush_color = color_code[1]
    def choose_size(self):
        size = simpledialog.askinteger("画笔粗细", "请输入画笔粗细 (1-50):",
                                       initialvalue=self.brush_size, minvalue=1, maxvalue=50)
        if size: self.brush_size = size

def ask_resolution(root):
    """弹出一个强制居中的模态对话框，让用户选择分辨率。"""
    dialog = tk.Toplevel(root)
    dialog.title("选择分辨率")
    
    # ### 核心修正：强制将对话框居中显示 ###
    dialog_width = 300
    dialog_height = 160
    screen_width = root.winfo_screenwidth()
    screen_height = root.winfo_screenheight()
    center_x = int(screen_width/2 - dialog_width / 2)
    center_y = int(screen_height/2 - dialog_height / 2)
    dialog.geometry(f'{dialog_width}x{dialog_height}+{center_x}+{center_y}')
    dialog.resizable(False, False) # 禁止调整大小
    
    dialog.transient(root)
    dialog.grab_set() # 强制用户必须先与此对话框交互

    tk.Label(dialog, text="这是您首次运行程序。\n请选择默认的画布分辨率：", pady=10).pack()
    result = tk.StringVar(value=None)

    def set_res_and_close(res):
        result.set(res)
        dialog.destroy()

    options_frame = tk.Frame(dialog)
    options_frame.pack(pady=5)
    
    tk.Button(options_frame, text="800x600", command=lambda: set_res_and_close("800x600"), width=10).pack(pady=2)
    tk.Button(options_frame, text="1280x720", command=lambda: set_res_and_close("1280x720"), width=10).pack(pady=2)
    tk.Button(options_frame, text="1600x900", command=lambda: set_res_and_close("1600x900"), width=10).pack(pady=2)

    def on_closing():
        if result.get() is None: result.set("800x600")
        dialog.destroy()
    
    dialog.protocol("WM_DELETE_WINDOW", on_closing)
    root.wait_window(dialog)
    return result.get()

def load_or_create_config(root):
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, 'r') as f: return json.load(f).get("resolution", "800x600")
        except: return create_config(root)
    else: return create_config(root)

def create_config(root):
    chosen_res = ask_resolution(root)
    config = {"resolution": chosen_res}
    with open(CONFIG_FILE, 'w') as f: json.dump(config, f)
    return chosen_res

def main():
    main_window = tk.Tk()
    main_window.withdraw()
    
    resolution = load_or_create_config(main_window)
    
    main_window.geometry(resolution)
    main_window.deiconify()
    
    app = DrawingApp(main_window)
    main_window.mainloop()

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        error_root = tk.Tk()
        error_root.withdraw()
        error_message = f"程序遇到致命错误，即将退出。\n\n错误信息:\n{e}\n\n详细追溯:\n{traceback.format_exc()}"
        messagebox.showerror("程序崩溃", error_message)
