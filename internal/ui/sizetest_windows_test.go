package ui

import (
	"testing"
	"unsafe"
)

func TestInputStructSize(t *testing.T) {
	if unsafe.Sizeof(input{}) != sizeofInput {
		t.Fatalf("sizeof(input)=%d, want %d", unsafe.Sizeof(input{}), sizeofInput)
	}
	if unsafe.Sizeof(keybdInput{}) != 24 {
		t.Fatalf("sizeof(keybdInput)=%d, want 24", unsafe.Sizeof(keybdInput{}))
	}
}
