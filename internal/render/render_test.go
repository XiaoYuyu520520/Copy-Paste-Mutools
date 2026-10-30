package render

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func netListen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func TestDecodePGM_P2AndP5(t *testing.T) {
	p2 := []byte("P2\n# 注释\n3 2\n255\n0 255 128  0 0 255\n")
	img, err := DecodePGM(p2)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 2 {
		t.Fatalf("尺寸错 %v", img.Bounds())
	}
	// 首行首列 0，第二行第三列 255
	if img.Pix[0] != 0 || img.Pix[5] != 255 || img.Pix[2] != 128 {
		t.Errorf("像素解析错误 %v", img.Pix)
	}

	var p5 strings.Builder
	p5.WriteString("P5\n2 2\n255\n")
	p5.Write([]byte{10, 20, 30, 40})
	img2, err := DecodePGM([]byte(p5.String()))
	if err != nil {
		t.Fatal(err)
	}
	if img2.Pix[3] != 40 {
		t.Errorf("P5 二进制像素错误 %v", img2.Pix)
	}

	if _, err := DecodePGM([]byte("P6\n1 1\n255\nx")); err == nil {
		t.Error("不支持的 P6 应报错")
	}
}

func TestParseYAML(t *testing.T) {
	y := `
# 元数据
image: map.pgm
resolution: 0.050000
origin: [-10.0, -5.5, 0.0]
negate: 0
occupied_thresh: 0.65
free_thresh: 0.196
`
	info, err := ParseYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if info.Image != "map.pgm" || info.Resolution != 0.05 {
		t.Errorf("字段解析错误: %+v", info)
	}
	if info.Origin[0] != -10 || info.Origin[1] != -5.5 {
		t.Errorf("origin 解析错误 %v", info.Origin)
	}
	if info.Negate != 0 || info.OccThresh != 0.65 || info.FreeThresh != 0.196 {
		t.Errorf("阈值解析错误 %+v", info)
	}
}

func TestPrepareMap(t *testing.T) {
	dir := t.TempDir()
	pgm := filepath.Join(dir, "lab.pgm")
	// 16x16 纯灰 128 的 P5
	data := []byte("P5\n16 16\n255\n")
	for i := 0; i < 16*16; i++ {
		data = append(data, 128)
	}
	if err := os.WriteFile(pgm, data, 0o644); err != nil {
		t.Fatal(err)
	}
	yaml := filepath.Join(dir, "lab.yaml")
	yamlBody := "image: lab.pgm\nresolution: 0.05\norigin: [0,0,0]\nnegate: 0\n"
	if err := os.WriteFile(yaml, []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := PrepareMap(pgm, yaml)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Width != 16 || doc.Height != 16 {
		t.Errorf("尺寸 %dx%d", doc.Width, doc.Height)
	}
	if doc.Png == "" || !strings.HasPrefix(doc.Png, "iVBOR") {
		t.Error("PNG base64 应为 iVBOR 开头")
	}
	if !doc.HasMetrics() || doc.Info.Resolution != 0.05 {
		t.Errorf("元数据丢失 %+v", doc.Info)
	}
	if doc.BaseName != "lab" {
		t.Errorf("BaseName=%q", doc.BaseName)
	}

	// 无 yaml 时仅预览
	doc2, err := PrepareMap(pgm, "")
	if err != nil {
		t.Fatal(err)
	}
	if doc2.HasMetrics() {
		t.Error("无 yaml 不应有度量")
	}
}

func TestLocalServerStopsAndReleasesPort(t *testing.T) {
	s := &LocalServer{}
	url, err := s.Start(http.NewServeMux())
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsRunning() {
		t.Fatal("应当处于运行态")
	}
	host := strings.TrimPrefix(url, "http://")
	s.Stop()
	if s.IsRunning() || s.URLSafe() != "" {
		t.Fatal("停止后状态应清空")
	}
	// 端口应已释放，可重新绑定
	if _, err := netListen(host); err != nil {
		t.Fatalf("停止后端口仍被占用: %v", err)
	}
}
