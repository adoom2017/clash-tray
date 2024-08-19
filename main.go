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
	cmd         *exec.Cmd
	logShow     *systray.MenuItem
	startClash  *systray.MenuItem
	stopClash   *systray.MenuItem
	quit        *systray.MenuItem
	clashStatus sync.Mutex
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

	ct.logShow = systray.AddMenuItem("Show Log", "Show logs")
	ct.startClash = systray.AddMenuItem("Start Clash", "Start clash app")
	ct.stopClash = systray.AddMenuItem("Stop Clash", "Stop clash app")
	ct.stopClash.Hide()
	systray.AddSeparator()
	ct.quit = systray.AddMenuItem("Quit", "Quit the app")
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
		case <-ct.quit.ClickedCh:
			ct.stopClashCmd()
			systray.Quit()
			return
		}
	}
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
	clashTray.cmd.Process.Kill()
}
