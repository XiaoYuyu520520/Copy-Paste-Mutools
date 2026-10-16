package ui

import (
	"testing"

	"fyne.io/fyne/v2/storage"
)

// TestLocalFilePath 原生窗口的拖放只认本地文件：远程 URI 与空 URI 一律忽略。
func TestLocalFilePath(t *testing.T) {
	if got := localFilePath(nil); got != "" {
		t.Errorf("nil URI 应返回空串，得到 %q", got)
	}
	if got := localFilePath(storage.NewURI("https://example.com/README.md")); got != "" {
		t.Errorf("远程 URI 应被忽略，得到 %q", got)
	}
	if got := localFilePath(storage.NewFileURI(`C:\demo\README.md`)); got == "" {
		t.Error("本地文件 URI 应解析出路径")
	}
}
