package main

import (
	"os"

	"clash-tray/log"

	"golang.org/x/sys/windows/registry"
)

const (
	// 注册表路径
	startupRegPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	// 程序在注册表中的键名
	appRegKeyName = "ClashTray"
)

// 检查程序是否已设置为开机自启动
func isAutoStartEnabled() bool {
    k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.QUERY_VALUE)
    if err != nil {
        return false
    }
    defer k.Close()

    val, _, err := k.GetStringValue(appRegKeyName)
    if err != nil {
        return false
    }

    // 确认路径是否匹配当前可执行文件路径
    exePath, err := os.Executable()
    if err != nil {
        return false
    }

    return val == exePath
}

// 启用自动启动
func enableAutoStart() error {
    k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer k.Close()

    exePath, err := os.Executable()
    if err != nil {
        return err
    }

    // 使用绝对路径设置自启动
    return k.SetStringValue(appRegKeyName, exePath)
}

// 禁用自动启动
func disableAutoStart() error {
    k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.SET_VALUE)
    if err != nil {
        return err
    }
    defer k.Close()

    return k.DeleteValue(appRegKeyName)
}

// 切换自动启动状态
func toggleAutoStart() bool {
    if isAutoStartEnabled() {
        if err := disableAutoStart(); err != nil {
            log.Errorln("禁用自动启动失败: %v", err)
            return true // 仍然启用
        }
        return false // 已禁用
    } else {
        if err := enableAutoStart(); err != nil {
            log.Errorln("启用自动启动失败: %v", err)
            return false // 仍然禁用
        }
        return true // 已启用
    }
}