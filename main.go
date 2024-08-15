package main

import (
	"bufio"
	"clash-ui/log"
	_ "embed"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/getlantern/systray"
)

//go:embed clash.ico
var icon []byte

var cmd *exec.Cmd = nil

func init() {
	logConfig := log.GetLogConfig()
	logConfig.Filename = "clash.log"

	log.SetLogConfig(logConfig)
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(icon)
	systray.SetTitle("Clash Tray App")
	systray.SetTooltip("Minimal Clash Windows To Windows Tray")

	startClash := systray.AddMenuItem("Start Clash", "Start Clash as admin")
	stopClash := systray.AddMenuItem("Stop Clash", "Stop Clash app")
	stopClash.Hide()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit the app")

	go func() {
		for {
			select {
			case <-startClash.ClickedCh:
				go startClashCmd()
				startClash.Hide()
				stopClash.Show()
			case <-stopClash.ClickedCh:
				stopClashCmd()
				stopClash.Hide()
				startClash.Show()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	stopClashCmd()
}

func startClashCmd() {
	cmd = exec.Command("./mihomo.exe", "-d", "./config")

	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}

	// 获取标准输出的管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorln("Falied to create StdoutPipe for clash: %v.", err)
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		log.Errorln("Falied to start clash: %v.", err)
		return
	}

	// 使用bufio.NewScanner读取输出
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Infoln(scanner.Text()) // 打印每一行输出
	}
}

func stopClashCmd() {
	if cmd == nil {
		return
	}

	cmd.Process.Kill()
}
