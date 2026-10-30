// Package shortcut 生成桌面网址快捷方式。
//
// 迁移自 legacy/python/小工具v1.0/code/桌面网址快捷方式生成器.py。
// 原脚本用 tkinter 做界面并生成 .bat 文件；这里改为：
//   - CLI 优先，GUI 入口放在 cmd/toolbox 的 paint/shortcut 交互模式
//   - Windows 生成 .bat（与原脚本一致）与 .url（双击行为更好）
//   - Linux 生成 .desktop
package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// illegalChars 是文件名中的非法字符（Windows 口径，与原脚本一致）。
var illegalChars = regexp.MustCompile(`[\\/:*?"<>|]`)

// Format 描述快捷方式的输出格式。
type Format string

const (
	// FormatAuto 按平台选择默认格式。
	FormatAuto Format = "auto"
	// FormatBat 生成 Windows 批处理快捷方式。
	FormatBat Format = "bat"
	// FormatURL 生成 Windows Internet Shortcut。
	FormatURL Format = "url"
	// FormatDesktop 生成 Linux .desktop 启动器。
	FormatDesktop Format = "desktop"
)

// Options 是创建快捷方式的参数。
type Options struct {
	// Name 是快捷方式名称（不含扩展名）。
	Name string
	// URL 是目标网址，必须以 http:// 或 https:// 开头。
	URL string
	// Dir 是保存目录。
	Dir string
	// Format 指定输出格式，默认按平台自动选择。
	Format Format
}

// Result 描述创建结果。
type Result struct {
	Path      string
	Content   string
	Overwrote bool
}

// Validate 校验参数并返回第一个错误。
func (o Options) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("快捷方式名称不能为空")
	}
	if strings.TrimSpace(o.URL) == "" {
		return fmt.Errorf("目标网址不能为空")
	}
	if !strings.HasPrefix(o.URL, "http://") && !strings.HasPrefix(o.URL, "https://") {
		return fmt.Errorf("请输入有效的网址（以 http:// 或 https:// 开头）")
	}
	if strings.TrimSpace(o.Dir) == "" {
		return fmt.Errorf("保存位置不能为空")
	}
	info, err := os.Stat(o.Dir)
	if err != nil {
		return fmt.Errorf("保存位置不存在: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("保存位置不是一个有效的文件夹")
	}
	return nil
}

// SafeName 去掉文件名里的非法字符。
func SafeName(name string) string {
	return illegalChars.ReplaceAllString(name, "")
}

// Create 按参数创建快捷方式文件。
func Create(opts Options) (Result, error) {
	if err := opts.Validate(); err != nil {
		return Result{}, err
	}

	safe := SafeName(opts.Name)
	if strings.TrimSpace(safe) == "" {
		return Result{}, fmt.Errorf("清理非法字符后名称为空")
	}

	format := opts.Format
	if format == "" || format == FormatAuto {
		format = defaultFormat()
	}

	var content, ext string
	switch format {
	case FormatBat:
		ext = ".bat"
		content = batContent(opts.URL)
	case FormatURL:
		ext = ".url"
		content = urlContent(opts.URL)
	case FormatDesktop:
		ext = ".desktop"
		content = desktopContent(opts.Name, opts.URL)
	default:
		return Result{}, fmt.Errorf("不支持的快捷方式格式: %s", format)
	}

	path := filepath.Join(opts.Dir, safe+ext)
	existed := false
	if _, err := os.Stat(path); err == nil {
		existed = true
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return Result{}, fmt.Errorf("写入失败: %w", err)
	}
	return Result{Path: path, Content: content, Overwrote: existed}, nil
}

// defaultFormat 按平台返回默认格式。
func defaultFormat() Format {
	if runtime.GOOS == "windows" {
		return FormatBat
	}
	return FormatDesktop
}

// batContent 生成 Windows 批处理内容。
//
// start 后的 "" 是为窗口指定空标题，防止 URL 里带空格时被当成标题。
func batContent(url string) string {
	return fmt.Sprintf("@echo off\r\nstart \"\" \"%s\"\r\n", url)
}

// urlContent 生成 Windows Internet Shortcut 内容。
func urlContent(url string) string {
	return fmt.Sprintf("[InternetShortcut]\r\nURL=%s\r\n", url)
}

// desktopContent 生成 Linux .desktop 启动器内容。
func desktopContent(name, url string) string {
	return fmt.Sprintf("[Desktop Entry]\r\nType=Link\r\nName=%s\r\nURL=%s\r\nIcon=internet-browser\r\nTerminal=false\r\n", name, url)
}

// DefaultDir 返回默认保存目录（桌面）。
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Desktop"), nil
	}
	// Linux 下桌面目录未必存在，回退到 home 本身。
	desktop := filepath.Join(home, "Desktop")
	if info, err := os.Stat(desktop); err == nil && info.IsDir() {
		return desktop, nil
	}
	return home, nil
}
