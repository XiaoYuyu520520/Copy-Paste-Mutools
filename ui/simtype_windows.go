//go:build windows

package ui

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard    = 1
	keyeventfKeyUp   = 0x0002
	keyeventfUnicode = 0x0004

	vkReturn  = 0x0D
	vkTab     = 0x09
	vkControl = 0x11
)

// keybdInput / input 对应 Windows 的 KEYBDINPUT / INPUT。
//
// 注意 sizeof(INPUT)：x64 下 union 取成员里最大的 MOUSEINPUT(32B)，
// 加上 type(DWORD)+对齐，INPUT 实为 40 字节——只按 KEYBDINPUT 算成 32 会
// 让 SendInput 返回 ERROR_INVALID_PARAMETER(0 事件)。末尾补 8 字节到 40。
// 运行前会校验 unsafe.Sizeof，若不是预期值直接报错而不是误发按键。
type keybdInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type input struct {
	typ uint32
	_   uint32
	ki  keybdInput
	_   uintptr // 补齐到 sizeof(INPUT)=40（x64）
}

// sizeofInput Windows INPUT 在 64 位下的字节数（type4+pad4+union 最大32）。
const sizeofInput = 40

func sendInputs(list []input) error {
	if len(list) == 0 {
		return nil
	}
	n, _, err := procSendInput.Call(
		uintptr(len(list)),
		uintptr(unsafe.Pointer(&list[0])),
		sizeofInput,
	)
	if int(n) != len(list) {
		return fmt.Errorf("SendInput 只写入了 %d/%d 个事件: %v", n, len(list), err)
	}
	return nil
}

// tapVK 发送一次普通虚拟键按下+抬起（Enter / Tab / 修饰键均可）。
func tapVK(vk uint16) error {
	down := input{typ: inputKeyboard, ki: keybdInput{WVk: vk}}
	up := input{typ: inputKeyboard, ki: keybdInput{WVk: vk, DwFlags: keyeventfKeyUp}}
	return sendInputs([]input{down, up})
}

// tapUnicode 用 KEYEVENTF_UNICODE 键入一个字符（支持中文等非 ASCII）。
// 对代理对（如 emoji）会逐 UTF-16 单元发送。
func tapUnicode(r rune) error {
	units := utf16.Encode([]rune{r})
	list := make([]input, 0, len(units)*2)
	for _, u := range units {
		list = append(list,
			input{typ: inputKeyboard, ki: keybdInput{WScan: u, DwFlags: keyeventfUnicode}},
			input{typ: inputKeyboard, ki: keybdInput{WScan: u, DwFlags: keyeventfUnicode | keyeventfKeyUp}},
		)
	}
	return sendInputs(list)
}

// tapCtrlEnter 模拟 Ctrl+Enter（聊天框常用「另起一行」而非直接发送）。
func tapCtrlEnter() error {
	list := []input{
		{typ: inputKeyboard, ki: keybdInput{WVk: vkControl}},
		{typ: inputKeyboard, ki: keybdInput{WVk: vkReturn}},
		{typ: inputKeyboard, ki: keybdInput{WVk: vkReturn, DwFlags: keyeventfKeyUp}},
		{typ: inputKeyboard, ki: keybdInput{WVk: vkControl, DwFlags: keyeventfKeyUp}},
	}
	return sendInputs(list)
}

// tapKey 按 simKey 类型发送按键。
func tapKey(k simKey) error {
	switch k.kind {
	case simKeyEnter:
		return tapVK(vkReturn)
	case simKeyTab:
		return tapVK(vkTab)
	case simKeyCtrlEnter:
		return tapCtrlEnter()
	case simKeyChar:
		return tapUnicode(k.r)
	}
	return errors.New("未知按键类型")
}

// waitOrStop 可中断的等待：到点返回 false；期间被 stopped() 则返回 true。
func waitOrStop(d time.Duration, stopped func() bool) bool {
	if d <= 0 {
		return stopped()
	}
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if stopped() {
			return true
		}
		time.Sleep(8 * time.Millisecond)
	}
	return false
}

// simulateKeys 按 planText 生成的序列做真实键盘输入。
//
// 参数：
//   - keys：待发送按键序列（重复 repeats 次）
//   - startDelay：开始前的等待，供用户切换到目标输入框
//   - perKey：每个字符后的间隔，防止输入过快被丢弃
//   - stopped：返回 true 表示用户要求停止
//   - step：每成功发送一个字符回调一次（在主调 goroutine 内执行）
//
// 返回 stopped=true 表示被用户中断（非错误）。
func simulateKeys(keys []simKey, startDelay, perKey time.Duration, repeats int, stopped func() bool, step func(int)) (bool, error) {
	if len(keys) == 0 {
		return false, nil
	}
	if unsafe.Sizeof(input{}) != sizeofInput {
		return false, errors.New("模拟键盘输入当前仅支持 64 位 Windows")
	}
	if repeats < 1 {
		repeats = 1
	}
	if waitOrStop(startDelay, stopped) {
		return true, nil
	}
	total := len(keys) * repeats
	done := 0
	for rep := 0; rep < repeats; rep++ {
		for _, k := range keys {
			if stopped() {
				return true, nil
			}
			if err := tapKey(k); err != nil {
				return false, err
			}
			done++
			step(done)
			if done >= total {
				return false, nil
			}
			if waitOrStop(perKey, stopped) {
				return true, nil
			}
		}
	}
	return false, nil
}
