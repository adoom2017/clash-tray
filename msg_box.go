package main

import (
	"syscall"
	"unsafe"
)

// 定义 Windows API 函数 MessageBox
var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

// MessageBox 调用
func MessageBox(title, caption string, boxType uint) int {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	captionPtr, _ := syscall.UTF16PtrFromString(caption)

	ret, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(unsafe.Pointer(captionPtr)),
		uintptr(boxType))
	return int(ret)
}
