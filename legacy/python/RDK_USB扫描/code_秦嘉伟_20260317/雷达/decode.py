import serial
import struct
import math

PORT = "COM3"
BAUD = 115200

DESC_INFO = b"\xA5\x5A\x14\x00\x00\x00\x04"
DESC_SCAN = b"\xA5\x5A\x05\x00\x00\x40\x81"
PH = b"\xAA\x55"


def angle_from_raw(v):
    return ((v >> 1) / 64.0) % 360.0


def dist_flag_from_raw(v):
    # 低2位是干扰标志，其余位是距离(mm)
    flag = v & 0x03
    dist = v >> 2
    return dist, flag


def parse_device_info(buf, idx):
    payload = buf[idx + 7: idx + 27]
    if len(payload) < 20:
        return None, idx

    model = payload[0]
    fw_major = payload[1]
    fw_minor = payload[2]
    hw = payload[3]
    sn = payload[4:20].hex(" ").upper()

    print("\n=== 设备信息 ===")
    print(f"Model      : 0x{model:02X}")
    print(f"Firmware   : {fw_major}.{fw_minor}")
    print(f"Hardware   : {hw}")
    print(f"Serial     : {sn}")
    print("================\n")

    return True, idx + 27


def parse_scan_packet(pkt):
    # pkt: 完整一帧，从 AA 55 开始
    if len(pkt) < 10:
        return None

    ph = pkt[0:2]
    ct = pkt[2]
    lsn = pkt[3]
    fsa = struct.unpack("<H", pkt[4:6])[0]
    lsa = struct.unpack("<H", pkt[6:8])[0]
    cs = struct.unpack("<H", pkt[8:10])[0]

    if ph != PH:
        return None

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

        points.append({
            "angle": angle,
            "distance_mm": dist,
            "flag": flag,
            "raw": raw
        })

    return {
        "ct": ct,
        "is_start": (ct & 0x01) == 1,
        "lsn": lsn,
        "start_angle": start_angle,
        "end_angle": end_angle,
        "cs": cs,
        "points": points
    }


def sniff_and_decode():
    print(f"正在连接 {PORT} @ {BAUD} ...")
    ser = serial.Serial(PORT, BAUD, timeout=0.2)
    print("串口已连接，开始解码...\n")

    buf = bytearray()
    seen_scan_desc = False

    try:
        while True:
            chunk = ser.read(4096)
            if chunk:
                buf.extend(chunk)

            # 设备信息
            while True:
                pos = buf.find(DESC_INFO)
                if pos == -1:
                    break
                ok, new_pos = parse_device_info(buf, pos)
                if ok is None:
                    break
                del buf[:new_pos]

            # 扫描描述符
            if not seen_scan_desc:
                pos = buf.find(DESC_SCAN)
                if pos != -1:
                    print("检测到扫描描述符，开始解析点云帧。\n")
                    del buf[:pos + len(DESC_SCAN)]
                    seen_scan_desc = True

            if not seen_scan_desc:
                continue

            # 解析 AA55 数据帧
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

                tag = "START" if parsed["is_start"] else "DATA"
                pts = parsed["points"]

                # 只打印非零点，避免刷屏
                valid_pts = [p for p in pts if p["distance_mm"] > 0]

                print(
                    f"[{tag}] CT=0x{parsed['ct']:02X} "
                    f"LSN={parsed['lsn']:02d} "
                    f"角度={parsed['start_angle']:.2f}° -> {parsed['end_angle']:.2f}° "
                    f"有效点={len(valid_pts)}"
                )

                if valid_pts:
                    preview = valid_pts[:8]
                    s = ", ".join(
                        f"({p['angle']:.1f}°, {p['distance_mm']}mm, flag={p['flag']})"
                        for p in preview
                    )
                    print("  ", s)

    except KeyboardInterrupt:
        print("\n停止。")
    finally:
        ser.close()
        print("串口已关闭。")


if __name__ == "__main__":
    sniff_and_decode()