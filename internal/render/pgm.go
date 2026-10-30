package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// 支持把 2D 激光 SLAM 建图结果（.pgm + .yaml）解析成可编辑/下载的地图文档。
// 单张 .pgm 也可预览（无 .yaml 时比例尺/测量自动禁用）。

// MapInfo 由 .yaml 解析出的元数据（map_server 通用字段）。
type MapInfo struct {
	Raw        string     `json:"raw"`
	Image      string     `json:"image"`
	Resolution float64    `json:"resolution"`
	Origin     [3]float64 `json:"origin"`
	Negate     int        `json:"negate"`
	OccThresh  float64    `json:"occupied_thresh"`
	FreeThresh float64    `json:"free_thresh"`
}

// MapDoc 浏览器地图页需要的内嵌数据。
type MapDoc struct {
	Png      string   `json:"png"`       // base64 编码的渲染图
	BaseName string   `json:"base_name"` // 保存时的文件名前缀
	Info     *MapInfo `json:"info"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
}

// hasMetrics 有完整元数据才启用比例尺/测量。
func (d *MapDoc) HasMetrics() bool {
	return d.Info != nil && d.Info.Resolution > 0
}

var originRe = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?`)

// ParseYAML 解析 map.yaml（容错缺失字段，原始文本保留）。
func ParseYAML(data []byte) (*MapInfo, error) {
	info := &MapInfo{
		Raw:        string(data),
		Negate:     0,
		OccThresh:  0.65,
		FreeThresh: 0.196,
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if i := strings.Index(val, "#"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		switch key {
		case "image":
			info.Image = strings.Trim(val, `"'`)
		case "resolution":
			info.Resolution, _ = strconv.ParseFloat(val, 64)
		case "origin":
			nums := originRe.FindAllString(val, -1)
			for i, n := range nums {
				if i >= len(info.Origin) {
					break
				}
				f, err := strconv.ParseFloat(n, 64)
				if err != nil {
					continue
				}
				info.Origin[i] = f
			}
		case "negate":
			n, _ := strconv.Atoi(val)
			info.Negate = n
		case "occupied_thresh":
			info.OccThresh, _ = strconv.ParseFloat(val, 64)
		case "free_thresh":
			info.FreeThresh, _ = strconv.ParseFloat(val, 64)
		}
	}
	return info, nil
}

// skipCommentTokens 从 pos 开始越过空白与 # 注释，返回下一个 token 起点。
func tokenize(data []byte, pos int) (tok string, next int, err error) {
	for pos < len(data) {
		c := data[pos]
		switch c {
		case ' ', '\t', '\r', '\n':
			pos++
		case '#':
			for pos < len(data) && data[pos] != '\n' {
				pos++
			}
		default:
			start := pos
			for pos < len(data) {
				c := data[pos]
				if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '#' {
					break
				}
				pos++
			}
			return string(data[start:pos]), pos, nil
		}
	}
	return "", pos, fmt.Errorf("意外到达文件末尾")
}

// DecodePGM 读取 P5/P2 PGM 为 8 位灰度图。
func DecodePGM(data []byte) (*image.Gray, error) {
	magic, pos, err := tokenize(data, 0)
	if err != nil {
		return nil, fmt.Errorf("解析 PGM 头失败: %w", err)
	}
	if magic != "P5" && magic != "P2" {
		return nil, fmt.Errorf("不支持的 PGM 魔数 %q（仅支持 P5/P2）", magic)
	}
	readInt := func() (int, error) {
		tok, p, e := tokenize(data, pos)
		if e != nil {
			return 0, e
		}
		pos = p
		v, e := strconv.Atoi(tok)
		return v, e
	}
	w, err := readInt()
	if err != nil {
		return nil, err
	}
	h, err := readInt()
	if err != nil {
		return nil, err
	}
	maxv, err := readInt()
	if err != nil {
		return nil, err
	}
	if w <= 0 || h <= 0 || maxv <= 0 {
		return nil, fmt.Errorf("非法尺寸/最大灰度: %dx%d max=%d", w, h, maxv)
	}
	img := image.NewGray(image.Rect(0, 0, w, h))
	scale := func(v int) uint8 {
		if maxv == 255 {
			return uint8(v)
		}
		// 按比例缩放（含 >255 的 16 位图）
		p := v * 255 / maxv
		if p > 255 {
			p = 255
		}
		return uint8(p)
	}

	// P5 的像素数据紧跟一个分隔空白字符（规范：恰好一个）之后开始
	if magic == "P5" && pos < len(data) {
		switch data[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		}
	}

	switch magic {
	case "P2":
		for i := 0; i < w*h; i++ {
			v, err := readInt()
			if err != nil {
				return nil, fmt.Errorf("P2 数据不足: %w", err)
			}
			img.Pix[i] = scale(v)
		}
	case "P5":
		if maxv <= 255 {
			if len(data)-pos < w*h {
				return nil, fmt.Errorf("P5 数据不足: 期望 %d 字节，实得 %d", w*h, len(data)-pos)
			}
			copy(img.Pix, data[pos:pos+w*h])
		} else {
			// 16 位大端
			for i := 0; i < w*h; i++ {
				if pos+2 > len(data) {
					return nil, fmt.Errorf("P5(16bit) 数据不足")
				}
				v := int(data[pos])<<8 | int(data[pos+1])
				pos += 2
				img.Pix[i] = scale(v)
			}
		}
	}
	return img, nil
}

// EncodePNG 把图像编码为 PNG 字节。
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PrepareMap 从 .pgm（+可选 .yaml）构建浏览器页内嵌数据。
func PrepareMap(pgmPath string, yamlPath string) (*MapDoc, error) {
	raw, err := os.ReadFile(pgmPath)
	if err != nil {
		return nil, err
	}
	gray, err := DecodePGM(raw)
	if err != nil {
		return nil, fmt.Errorf("解析 %s: %w", filepath.Base(pgmPath), err)
	}
	pngBytes, err := EncodePNG(gray)
	if err != nil {
		return nil, err
	}
	doc := &MapDoc{
		Png:      base64.StdEncoding.EncodeToString(pngBytes),
		BaseName: strings.TrimSuffix(filepath.Base(pgmPath), filepath.Ext(pgmPath)),
		Width:    gray.Bounds().Dx(),
		Height:   gray.Bounds().Dy(),
	}
	if yamlPath != "" {
		y, err := os.ReadFile(yamlPath)
		if err == nil {
			info, err := ParseYAML(y)
			if err == nil {
				doc.Info = info
				// 保存名跟随 yaml 里的 image 名，若 yaml 缺失则由 pgm 名兜底
				if info.Image != "" {
					doc.BaseName = strings.TrimSuffix(filepath.Base(info.Image), filepath.Ext(info.Image))
				}
			}
		}
	}
	return doc, nil
}
