package main

import (
	"bufio"
	"clash-tray/log"
	_ "embed"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

//go:embed app-disable.ico
var iconDisable []byte

//go:embed app-enable.ico
var iconEnable []byte

const (
	defaultLogFileName = "mihomo.log"
	exeName            = "./mihomo.exe"
	configDir          = "./config"
)

type clashTrayS struct {
	cmd        *exec.Cmd
	logShow    *systray.MenuItem
	startClash *systray.MenuItem
	stopClash  *systray.MenuItem

	// 新增: 更新Mihomo菜单项
	updateMihomo  *systray.MenuItem
	autoStartItem *systray.MenuItem

	quit        *systray.MenuItem
	clashStatus sync.Mutex

	// 更新状态标志
	isUpdating bool
}

var clashTray *clashTrayS = nil

func init() {
	logConfig := log.GetLogConfig()
	logConfig.Filename = defaultLogFileName

	log.SetLogConfig(logConfig)
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	clashTray = &clashTrayS{}

	clashTray.setupMenu()
	go clashTray.handleEvents()
}

func (ct *clashTrayS) setupMenu() {
	systray.SetIcon(iconDisable)
	systray.SetTitle("Clash Tray App")
	systray.SetTooltip("Minimal clash command window to tray")

	ct.logShow = systray.AddMenuItem("显示日志", "Show logs")
	ct.startClash = systray.AddMenuItem("启动 Clash", "Start clash app")
	ct.stopClash = systray.AddMenuItem("停止 Clash", "Stop clash app")
	ct.stopClash.Hide()
	systray.AddSeparator()

	// 新增: 更新Mihomo菜单项
	ct.updateMihomo = systray.AddMenuItem("更新Mihomo", "从GitHub下载最新版本的Mihomo")

	// 添加开机自启动菜单
	ct.autoStartItem = systray.AddMenuItemCheckbox("开机自启动", "设置开机自动启动", isAutoStartEnabled())

	systray.AddSeparator()
	ct.quit = systray.AddMenuItem("退出", "Quit the app")
}

func (ct *clashTrayS) handleEvents() {
	for {
		select {
		case <-ct.startClash.ClickedCh:
			ct.startClashCmd()
		case <-ct.stopClash.ClickedCh:
			ct.stopClashCmd()
		case <-ct.logShow.ClickedCh:
			showLog()
		case <-ct.updateMihomo.ClickedCh:
			ct.handleUpdateMihomo()
		case <-ct.autoStartItem.ClickedCh:
			enabled := toggleAutoStart()
			if enabled {
				ct.autoStartItem.Check()
			} else {
				ct.autoStartItem.Uncheck()
			}
		case <-ct.quit.ClickedCh:
			ct.stopClashCmd()
			systray.Quit()
			return
		}
	}
}

// 处理更新Mihomo的逻辑
func (ct *clashTrayS) handleUpdateMihomo() {

	// 防止重复点击
	if ct.isUpdating {
		MessageBox("更新已在进行中，请稍候...", "提示", 0x00000040)
		return
	}

	// 检查Clash是否正在运行
	ct.clashStatus.Lock()
	isRunning := ct.cmd != nil
	ct.clashStatus.Unlock()

	// 更改菜单项状态
	ct.isUpdating = true
	ct.updateMihomo.SetTitle("正在下载更新...")
	ct.updateMihomo.Disable()

	// 在后台执行更新
	go func() {
		// 第一步：下载更新包到临时文件
		zipPath, err := DownloadLatestMihomoToTemp()

		if err != nil {
			// 下载失败，恢复菜单状态
			ct.isUpdating = false
			ct.updateMihomo.SetTitle("更新Mihomo")
			ct.updateMihomo.Enable()

			log.Errorln("下载更新失败: %v", err)
			MessageBox("下载更新失败: "+err.Error(), "错误", 0x00000010)
			return
		}

		// 第二步：如果程序正在运行，停止程序
		if isRunning {
			ct.updateMihomo.SetTitle("正在停止服务...")
			log.Infoln("停止Mihomo服务以进行更新...")
			ct.stopClashCmd()
			// 等待程序完全停止
			time.Sleep(1 * time.Second)
		}

		// 第三步：应用更新（解压并替换）
		ct.updateMihomo.SetTitle("正在应用更新...")
		err = ApplyUpdate(zipPath)

		// 恢复菜单项状态
		ct.isUpdating = false
		ct.updateMihomo.SetTitle("更新Mihomo")
		ct.updateMihomo.Enable()

		// 显示结果
		if err != nil {
			log.Errorln("应用更新失败: %v", err)
			MessageBox("应用更新失败: "+err.Error(), "错误", 0x00000010)
			// 如果之前在运行，询问是否重新启动旧版本
			if isRunning {
				result := MessageBox("更新失败，是否重新启动服务？", "确认", 0x00000004)
				if result == 6 { // IDYES
					ct.startClashCmd()
				}
			}
		} else {
			log.Infoln("Mihomo更新成功！")
			// 第四步：如果之前在运行，自动重新启动
			if isRunning {
				MessageBox("Mihomo更新成功！正在重新启动服务...", "成功", 0x00000040)
				time.Sleep(500 * time.Millisecond)
				ct.startClashCmd()
			} else {
				MessageBox("Mihomo更新成功！", "成功", 0x00000040)
			}
		}
	}()
}

func (ct *clashTrayS) startClashCmd() {
	ct.clashStatus.Lock()
	defer ct.clashStatus.Unlock()

	if ct.cmd != nil {
		return
	}

	ct.cmd = exec.Command(exeName, "-d", configDir)

	if runtime.GOOS == "windows" {
		ct.cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	stdout, err := ct.cmd.StdoutPipe()
	if err != nil {
		log.Errorln("Failed to create StdoutPipe for clash: %v", err)
		MessageBox(err.Error(), "Failed to start clash", 0x00000010)
		ct.cmd = nil
		return
	}

	if err := ct.cmd.Start(); err != nil {
		log.Errorln("Failed to start clash: %v", err)
		MessageBox("启动失败: "+err.Error(), "Failed", 0x00000010)
		ct.cmd = nil
		return
	}

	ct.startClash.Hide()
	ct.stopClash.Show()
	systray.SetIcon(iconEnable)

	MessageBox("启动成功", "Succeed", 0x00000000)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Infoln(scanner.Text())
		}
	}()

	go func() {
		err := ct.cmd.Wait()
		ct.clashStatus.Lock()
		defer ct.clashStatus.Unlock()
		ct.cmd = nil
		ct.startClash.Show()
		ct.stopClash.Hide()
		systray.SetIcon(iconDisable)
		if err != nil {
			log.Errorln("Clash exited with error: %v", err)
		}
	}()
}

func (ct *clashTrayS) stopClashCmd() {
	ct.clashStatus.Lock()
	defer ct.clashStatus.Unlock()

	if ct.cmd == nil || ct.cmd.Process == nil {
		return
	}

	if err := ct.cmd.Process.Kill(); err != nil {
		log.Errorln("Failed to stop clash: %v", err)
	}
}

func showLog() {
	if err := exec.Command("notepad.exe", defaultLogFileName).Start(); err != nil {
		log.Errorln("Failed to open log file: %v", err)
	}
}

func onExit() {
	if clashTray != nil && clashTray.cmd != nil && clashTray.cmd.Process != nil {
		clashTray.cmd.Process.Kill()
	}
}
