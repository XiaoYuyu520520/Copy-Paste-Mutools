package paint

import (
	"image"
	"image/png"
	"io"
)

// encodePNG 把图像编码成 PNG。
func encodePNG(w io.Writer, img image.Image) error {
	return png.Encode(w, img)
}
