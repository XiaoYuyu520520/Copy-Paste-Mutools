// mkicon 把来源 .ico 里的最大图像重新编码为一组合法尺寸的多帧 .ico（全 32 位 DIB），
// 并导出一张 icon.png 供 Fyne 运行时设置应用/任务栏图标。
//
// 用法：go run ./cmd/mkicon -src ico/default_icon.ico [-out ico/default_icon.ico] [-png ico/icon.png]
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	xdraw "golang.org/x/image/draw"
)

func u16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func u32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func putU16(b []byte, v uint16) {
	binary.LittleEndian.PutUint16(b, v)
}
func putU32(b []byte, v uint32) {
	binary.LittleEndian.PutUint32(b, v)
}

func icoDim(v byte) int {
	if v == 0 {
		return 256
	}
	return int(v)
}

// readSource 解析 ICO，返回最大的可解码图像（优先 PNG 帧）。
func readSource(path string) (image.Image, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 6 || u16(raw[0:]) != 0 || u16(raw[2:]) != 1 {
		return nil, fmt.Errorf("不是有效的 ICO 文件: %s", path)
	}
	count := int(u16(raw[4:]))
	var best image.Image
	bestSize := 0
	for i := 0; i < count; i++ {
		off := 6 + i*16
		if off+16 > len(raw) {
			break
		}
		e := raw[off : off+16]
		w, h := icoDim(e[0]), icoDim(e[1])
		if w*h < bestSize {
			continue
		}
		dataOff := int(u32(e[12:]))
		dataLen := int(u32(e[8:]))
		if dataOff+dataLen > len(raw) {
			continue
		}
		data := raw[dataOff : dataOff+dataLen]
		var img image.Image
		if len(data) > 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
			if im, err := png.Decode(bytes.NewReader(data)); err == nil {
				img = im
			}
		} else {
			img, err = decodeDIBFrame(data, w)
			if err != nil {
				continue
			}
		}
		if img != nil {
			b := img.Bounds()
			if b.Dx()*b.Dy() > bestSize {
				bestSize = b.Dx() * b.Dy()
				best = img
			}
		}
	}
	if best == nil {
		return nil, fmt.Errorf("未能从 %s 解码出图像帧", path)
	}
	return best, nil
}

// decodeDIBFrame 解码 ICO 内的 32/24 位 DIB 帧（含 AND 掩码）。
func decodeDIBFrame(d []byte, expect int) (image.Image, error) {
	if len(d) < 40 || int(u32(d[0:])) < 40 {
		return nil, fmt.Errorf("DIB 头过短")
	}
	biSize := int(u32(d[0:]))
	w := int(int32(u32(d[4:])))
	hField := int32(u32(d[8:]))
	h := int(hField)
	topDown := h < 0
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 || w > 1024 || h > 1024 {
		return nil, fmt.Errorf("非法尺寸 %dx%d", w, h)
	}
	planes := u16(d[12:])
	bpp := u16(d[14:])
	if planes != 1 || (bpp != 32 && bpp != 24) {
		return nil, fmt.Errorf("不支持的位深 %d", bpp)
	}
	start := biSize
	if u32(d[16:]) == 3 { // BI_BITFIELDS
		start += 12
	}
	// ICO 高度约定常为 图像高度*2（含 AND 掩码）
	rowBytes := w * int(bpp/8)
	if start+rowBytes*h > len(d) {
		halved := h / 2
		if start+rowBytes*halved <= len(d) {
			h = halved
		} else {
			return nil, fmt.Errorf("像素数据不足")
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		var srcY int
		if topDown {
			srcY = y
		} else {
			srcY = h - 1 - y
		}
		base := start + srcY*rowBytes
		for x := 0; x < w; x++ {
			p := base + x*int(bpp/8)
			b := d[p]
			g := d[p+1]
			r := d[p+2]
			a := uint8(255)
			if bpp == 32 {
				a = d[p+3]
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img, nil
}

func encodeDIBFrame(im image.Image, size int) ([]byte, error) {
	src := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.BiLinear.Scale(src, src.Bounds(), im, im.Bounds(), xdraw.Over, nil)
	rowBytes := size * 4
	maskRow := ((size + 31) / 32) * 4
	height := size * 2 // ICO：高度含掩码
	buf := make([]byte, 40+rowBytes*size+maskRow*size)
	putU32(buf[0:], 40)
	putU32(buf[4:], uint32(size))
	putU32(buf[8:], uint32(height))
	putU16(buf[12:], 1)
	putU16(buf[14:], 32)
	pos := 40
	for y := size - 1; y >= 0; y-- { // 自下而上
		for x := 0; x < size; x++ {
			c := src.RGBAAt(x, y)
			buf[pos] = c.B
			buf[pos+1] = c.G
			buf[pos+2] = c.R
			buf[pos+3] = c.A
			pos += 4
		}
	}
	pos += size * maskRow // 掩码区（全 0 = 不透明）
	return buf, nil
}

func main() {
	src := flag.String("src", "ico/default_icon.ico", "源 .ico")
	out := flag.String("out", "ico/default_icon.ico", "输出 .ico")
	pngOut := flag.String("png", "ico/icon.png", "输出 icon.png")
	flag.Parse()

	base, err := readSource(*src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取源图标失败:", err)
		os.Exit(1)
	}
	sizes := []int{16, 24, 32, 48, 64, 128, 256}

	var header bytes.Buffer
	frames := make([][]byte, len(sizes))
	offsets := make([]int, len(sizes))
	cur := 6 + len(sizes)*16
	for i, s := range sizes {
		f, err := encodeDIBFrame(base, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, "编码失败:", err)
			os.Exit(1)
		}
		frames[i] = f
		offsets[i] = cur
		cur += len(f)
	}

	_ = binary.Write(&header, binary.LittleEndian, uint16(0))
	_ = binary.Write(&header, binary.LittleEndian, uint16(1))
	_ = binary.Write(&header, binary.LittleEndian, uint16(len(sizes)))
	for i, s := range sizes {
		dim := byte(s)
		if s == 256 {
			dim = 0
		}
		entry := make([]byte, 16)
		entry[0], entry[1] = dim, dim
		putU16(entry[4:], 1)
		putU16(entry[6:], 32)
		putU32(entry[8:], uint32(len(frames[i])))
		putU32(entry[12:], uint32(offsets[i]))
		header.Write(entry)
	}
	var all bytes.Buffer
	all.Write(header.Bytes())
	for _, f := range frames {
		all.Write(f)
	}
	if err := os.WriteFile(*out, all.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "写 ico 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("已写入 %s (%.1f KB)\n", *out, float64(all.Len())/1024)

	big := image.NewRGBA(image.Rect(0, 0, 256, 256))
	xdraw.BiLinear.Scale(big, big.Bounds(), base, base.Bounds(), xdraw.Over, nil)
	f, err := os.Create(*pngOut)
	if err != nil {
		fmt.Fprintln(os.Stderr, "创建 png 失败:", err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, big); err != nil {
		fmt.Fprintln(os.Stderr, "写 png 失败:", err)
		os.Exit(1)
	}
	fmt.Printf("已导出 %s\n", *pngOut)
}
