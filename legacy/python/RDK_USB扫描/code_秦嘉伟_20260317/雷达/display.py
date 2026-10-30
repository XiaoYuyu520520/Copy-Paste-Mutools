import math
import struct
import threading
import time
from collections import deque

import pygame
import serial


# =========================
# 串口/协议参数
# =========================
PORT = "COM3"
BAUD = 115200

DESC_INFO = b"\xA5\x5A\x14\x00\x00\x00\x04"
DESC_SCAN = b"\xA5\x5A\x05\x00\x00\x40\x81"
PH = b"\xAA\x55"

# =========================
# 可视化参数
# =========================
WIDTH = 1000
HEIGHT = 800
FPS = 60

MAX_DISTANCE_MM = 4000       # 最大显示距离，超出就裁剪
MIN_VALID_DISTANCE_MM = 1    # 小于这个就不画
POINT_RADIUS = 2

# 角度显示偏移
# 0 表示协议角度 그대로
# 如果想让 0° 朝上，可以用 -90
ANGLE_OFFSET_DEG = -90

# 是否镜像 X 轴 / Y 轴
FLIP_X = False
FLIP_Y = True

# 每圈结束后保留多少历史圈做淡化
HISTORY_SWEEPS = 3


# =========================
# 共享数据
# =========================
data_lock = threading.Lock()
latest_sweep = []               # 当前显示的一整圈 [(angle_deg, dist_mm), ...]
history_sweeps = deque(maxlen=HISTORY_SWEEPS)
current_scan_angle = None
device_info = {}
running = True


# =========================
# 协议解析
# =========================
def angle_from_raw(v: int) -> float:
    return ((v >> 1) / 64.0) % 360.0


def dist_flag_from_raw(v: int):
    flag = v & 0x03
    dist = v >> 2
    return dist, flag


def parse_device_info_payload(payload: bytes):
    if len(payload) < 20:
        return None
    return {
        "model": payload[0],
        "fw_major": payload[1],
        "fw_minor": payload[2],
        "hardware": payload[3],
        "serial": payload[4:20].hex(" ").upper(),
    }


def parse_scan_packet(pkt: bytes):
    if len(pkt) < 10:
        return None

    if pkt[0:2] != PH:
        return None

    ct = pkt[2]
    lsn = pkt[3]
    fsa = struct.unpack("<H", pkt[4:6])[0]
    lsa = struct.unpack("<H", pkt[6:8])[0]
    cs = struct.unpack("<H", pkt[8:10])[0]

    expected_len = 10 + lsn * 2
    if len(pkt) != expected_len:
        return None

    start_angle = angle_from_raw(fsa)
    end_angle = angle_from_raw(lsa)

    points = []
    for i in range(lsn):
        raw = struct.unpack("<H", pkt[10 + i * 2: 12 + i * 2])[0]
        dist, flag = dist_flag_from_raw(raw)

        if lsn == 1:
            angle = start_angle
        else:
            diff = end_angle - start_angle
            if diff < 0:
                diff += 360.0
            angle = (start_angle + diff * i / (lsn - 1)) % 360.0

        points.append((angle, dist, flag))

    return {
        "ct": ct,
        "is_start": (ct & 0x01) == 1,
        "lsn": lsn,
        "start_angle": start_angle,
        "end_angle": end_angle,
        "cs": cs,
        "points": points,
    }


# =========================
# 后台串口线程
# =========================
def serial_worker():
    global running, latest_sweep, current_scan_angle, device_info

    buf = bytearray()
    seen_scan_desc = False
    current_sweep = []

    try:
        ser = serial.Serial(PORT, BAUD, timeout=0.05)
        print(f"[INFO] 已连接 {PORT} @ {BAUD}")
    except Exception as e:
        print(f"[ERROR] 串口打开失败: {e}")
        running = False
        return

    try:
        while running:
            chunk = ser.read(4096)
            if chunk:
                buf.extend(chunk)

            # 设备信息描述符
            while True:
                pos = buf.find(DESC_INFO)
                if pos == -1:
                    break

                if len(buf) < pos + 27:
                    break

                payload = bytes(buf[pos + 7: pos + 27])
                info = parse_device_info_payload(payload)
                if info:
                    with data_lock:
                        device_info = info
                    print("[INFO] Device info:", info)

                del buf[:pos + 27]

            # 扫描描述符
            if not seen_scan_desc:
                pos = buf.find(DESC_SCAN)
                if pos != -1:
                    seen_scan_desc = True
                    del buf[:pos + len(DESC_SCAN)]
                    print("[INFO] 检测到扫描流")
                else:
                    if len(buf) > 8192:
                        del buf[:-16]
                    continue

            # 解析数据帧
            while True:
                head = buf.find(PH)
                if head == -1:
                    if len(buf) > 8192:
                        del buf[:-2]
                    break

                if head > 0:
                    del buf[:head]

                if len(buf) < 10:
                    break

                lsn = buf[3]
                frame_len = 10 + lsn * 2
                if len(buf) < frame_len:
                    break

                frame = bytes(buf[:frame_len])
                del buf[:frame_len]

                parsed = parse_scan_packet(frame)
                if not parsed:
                    continue

                with data_lock:
                    current_scan_angle = parsed["end_angle"]

                # 一圈开始：把上一圈提交出去
                if parsed["is_start"]:
                    if current_sweep:
                        with data_lock:
                            history_sweeps.append(list(latest_sweep))
                            latest_sweep = list(current_sweep)
                        current_sweep.clear()

                for angle, dist, flag in parsed["points"]:
                    if dist < MIN_VALID_DISTANCE_MM:
                        continue
                    if dist > MAX_DISTANCE_MM:
                        dist = MAX_DISTANCE_MM
                    current_sweep.append((angle, dist))

    except Exception as e:
        print(f"[ERROR] 串口线程异常: {e}")
    finally:
        try:
            ser.close()
        except Exception:
            pass
        print("[INFO] 串口已关闭")


# =========================
# 绘图函数
# =========================
def polar_to_screen(angle_deg, dist_mm, center, radius_px):
    cx, cy = center

    r = min(dist_mm / MAX_DISTANCE_MM, 1.0) * radius_px
    a = math.radians(angle_deg + ANGLE_OFFSET_DEG)

    x = cx + r * math.cos(a)
    y = cy + r * math.sin(a)

    if FLIP_X:
        x = cx - (x - cx)
    if FLIP_Y:
        y = cy - (y - cy)

    return int(x), int(y)


def draw_grid(screen, center, radius_px, font):
    cx, cy = center

    # 同心圆
    for i in range(1, 5):
        r = int(radius_px * i / 4)
        pygame.draw.circle(screen, (50, 70, 50), center, r, 1)

        dist_label = int(MAX_DISTANCE_MM * i / 4)
        txt = font.render(f"{dist_label} mm", True, (110, 160, 110))
        screen.blit(txt, (cx + 6, cy - r - 14))

    # 十字线
    pygame.draw.line(screen, (60, 90, 60), (cx - radius_px, cy), (cx + radius_px, cy), 1)
    pygame.draw.line(screen, (60, 90, 60), (cx, cy - radius_px), (cx, cy + radius_px), 1)

    # 45° 射线
    for deg in range(0, 360, 45):
        x, y = polar_to_screen(deg, MAX_DISTANCE_MM, center, radius_px)
        pygame.draw.line(screen, (40, 60, 40), center, (x, y), 1)

        lx, ly = polar_to_screen(deg, MAX_DISTANCE_MM + 150, center, radius_px)
        label = font.render(str(deg), True, (100, 150, 100))
        rect = label.get_rect(center=(lx, ly))
        screen.blit(label, rect)

    pygame.draw.circle(screen, (0, 255, 120), center, 4)


def draw_points(screen, points, center, radius_px, color, point_radius):
    for angle, dist in points:
        x, y = polar_to_screen(angle, dist, center, radius_px)
        pygame.draw.circle(screen, color, (x, y), point_radius)


def main():
    global running

    # 启动串口线程
    t = threading.Thread(target=serial_worker, daemon=True)
    t.start()

    pygame.init()
    pygame.display.set_caption("Radar Real-time Viewer")
    screen = pygame.display.set_mode((WIDTH, HEIGHT))
    clock = pygame.time.Clock()

    font = pygame.font.SysFont("consolas", 18)
    small_font = pygame.font.SysFont("consolas", 14)

    center = (WIDTH // 2, HEIGHT // 2)
    radius_px = min(WIDTH, HEIGHT) // 2 - 80

    zoom_scale = 1.0

    while running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False

            elif event.type == pygame.KEYDOWN:
                if event.key == pygame.K_ESCAPE:
                    running = False
                elif event.key in (pygame.K_EQUALS, pygame.K_PLUS, pygame.K_KP_PLUS):
                    zoom_scale *= 1.1
                elif event.key in (pygame.K_MINUS, pygame.K_KP_MINUS):
                    zoom_scale /= 1.1
                    zoom_scale = max(0.2, zoom_scale)
                elif event.key == pygame.K_r:
                    zoom_scale = 1.0

        screen.fill((8, 18, 8))

        scaled_radius = int(radius_px * zoom_scale)
        draw_grid(screen, center, scaled_radius, small_font)

        with data_lock:
            info = dict(device_info)
            sweep = list(latest_sweep)
            hist = [list(s) for s in history_sweeps]
            scan_angle = current_scan_angle

        # 画历史圈
        history_colors = [
            (30, 80, 30),
            (40, 120, 40),
            (60, 160, 60),
        ]
        for i, old_sweep in enumerate(hist[-HISTORY_SWEEPS:]):
            color = history_colors[min(i, len(history_colors) - 1)]
            draw_points(screen, old_sweep, center, scaled_radius, color, 1)

        # 当前圈
        draw_points(screen, sweep, center, scaled_radius, (0, 255, 120), POINT_RADIUS)

        # 当前扫描方向
        if scan_angle is not None:
            x, y = polar_to_screen(scan_angle, MAX_DISTANCE_MM, center, scaled_radius)
            pygame.draw.line(screen, (0, 180, 255), center, (x, y), 2)

        # HUD
        lines = [
            f"Port: {PORT}  Baud: {BAUD}",
            f"Points in sweep: {len(sweep)}",
            f"Max distance: {MAX_DISTANCE_MM} mm",
            f"Zoom: {zoom_scale:.2f}x   [+/-]  reset[R]",
        ]

        if info:
            lines.append(
                f"Model: 0x{info.get('model', 0):02X}  FW: {info.get('fw_major', 0)}.{info.get('fw_minor', 0)}  HW: {info.get('hardware', 0)}"
            )

        for i, line in enumerate(lines):
            txt = font.render(line, True, (180, 255, 180))
            screen.blit(txt, (20, 20 + i * 24))

        tip = small_font.render("ESC / Close window to exit", True, (120, 180, 120))
        screen.blit(tip, (20, HEIGHT - 30))

        pygame.display.flip()
        clock.tick(FPS)

    pygame.quit()
    time.sleep(0.2)


if __name__ == "__main__":
    main()