package main

import (
	"bufio"
	"clash-tray/log"
	_ "embed"
	"os/exec"
	"runtime"
	"sync"
	"syscall"

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

	// 如果Clash正在运行，先提示用户
	ct.clashStatus.Lock()
	isRunning := ct.cmd != nil
	ct.clashStatus.Unlock()

	if isRunning {
		result := MessageBox("更新需要先停止Mihomo服务，是否继续？", "确认", 0x00000004)
		if result != 6 { // 6 = IDYES
			return
		}
		// 停止Clash
		ct.stopClashCmd()
	}

	// 更改菜单项状态
	ct.isUpdating = true
	ct.updateMihomo.SetTitle("正在更新...")
	ct.updateMihomo.Disable()

	// 在后台执行更新
	go func() {
		err := DownloadLatestMihomo()

		// 恢复菜单项状态
		ct.isUpdating = false
		ct.updateMihomo.SetTitle("更新Mihomo")
		ct.updateMihomo.Enable()

		// 显示结果
		if err != nil {
			log.Errorln("更新失败: %v", err)
			MessageBox("更新失败: "+err.Error(), "错误", 0x00000010)
		} else {
			MessageBox("Mihomo更新成功！", "成功", 0x00000040)
			// 如果之前在运行，则重新启动
			if isRunning {
				result := MessageBox("是否立即启动Mihomo服务？", "确认", 0x00000004)
				if result == 6 { // IDYES
					ct.startClashCmd()
				}
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
