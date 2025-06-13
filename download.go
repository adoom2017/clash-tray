package main

import (
	"archive/zip"
	"clash-tray/log"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// GitHub API地址
	mihomoRepoAPI = "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
	// 临时文件夹
	tempDir = "./temp"
	// 下载超时时间
	downloadTimeout = 60 * time.Second

	// 架构
	arch = "amd64"

	// 平台
	platform = "windows"
)

// GitHub API 响应结构
type GithubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// 检查当前mihomo版本
func getCurrentVersion() (string, error) {
	// 检查mihomo是否存在
	if _, err := os.Stat(exeName); err != nil {
		return "", fmt.Errorf("未找到mihomo可执行文件: %w", err)
	}

	// 执行mihomo -v命令获取版本信息
	cmd := exec.Command(exeName, "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取版本信息失败: %w", err)
	}

	// 使用正则表达式提取版本号
	// 匹配格式如 "v1.19.10"，即字母v后跟数字、点的组合
	re := regexp.MustCompile(`v\d+(\.\d+)+`)
	versionMatch := re.FindString(string(output))

	if versionMatch == "" {
		return "", fmt.Errorf("无法从输出中提取版本号: %s", string(output))
	}

	return versionMatch, nil
}

// 下载Mihomo最新版本
func DownloadLatestMihomo() error {
	// 创建一个带超时的HTTP客户端
	client := &http.Client{
		Timeout: downloadTimeout,
	}

	// 获取最新版本信息
	log.Infoln("正在获取Mihomo最新版本信息...")
	resp, err := client.Get(mihomoRepoAPI)
	if err != nil {
		return fmt.Errorf("获取版本信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析API响应
	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("解析API响应失败: %w", err)
	}

	log.Infoln("找到最新版本: %s", release.TagName)

	// 获取当前版本
	currentVersion, err := getCurrentVersion()
	if err != nil {
		log.Warnln("无法获取当前版本: %v，将继续更新", err)
	} else {
		// 比较版本，如果相同则不更新
		if currentVersion == release.TagName {
			log.Infoln("当前已是最新版本 %s，无需更新", currentVersion)
			return fmt.Errorf("当前已是最新版本 %s，无需更新", currentVersion)
		}
		log.Infoln("当前版本: %s, 最新版本: %s", currentVersion, release.TagName)
	}

	// 确定适合当前系统的资源
	assetURL := ""
	assetName := ""

	nameToFind := fmt.Sprintf("mihomo-%s-%s-compatible-%s.zip", platform, arch, release.TagName)

	// 查找匹配的资源
	for _, asset := range release.Assets {
		// 寻找Windows版本且匹配当前架构
		if strings.Contains(asset.Name, nameToFind) {
			assetURL = asset.BrowserDownloadURL
			assetName = asset.Name
			break
		}
	}

	if assetURL == "" {
		return fmt.Errorf("找不到适合当前系统的下载资源")
	}

	log.Infoln("准备下载: %s", assetName)

	// 确保临时目录存在
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 下载文件路径
	zipPath := filepath.Join(tempDir, assetName)

	// 下载文件
	if err := downloadFile(assetURL, zipPath, client); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	log.Infoln("下载完成，开始解压...")

	// 解压缩zip文件
	if err := extractMihomo(zipPath); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	log.Infoln("更新完成！")

	// 清理临时文件
	os.RemoveAll(tempDir)

	return nil
}

// 下载文件到指定路径
func downloadFile(url, filePath string, client *http.Client) error {
	// 创建一个请求对象
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 设置User-Agent，避免GitHub API限制
	req.Header.Set("User-Agent", "Clash-Tray")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载请求失败，状态码: %d", resp.StatusCode)
	}

	// 创建目标文件
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 将响应体复制到文件
	_, err = io.Copy(out, resp.Body)
	return err
}

// 从zip文件中提取mihomo.exe
func extractMihomo(zipPath string) error {
	// 打开zip文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 遍历zip文件中的所有文件
	for _, f := range r.File {
		// 找到mihomo.exe文件
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			// 打开zip中的文件
			src, err := f.Open()
			if err != nil {
				return err
			}
			defer src.Close()

			// 创建目标文件（备份旧文件）
			if _, err := os.Stat(exeName); err == nil {
				backupPath := exeName + ".bak"
				if err := os.Rename(exeName, backupPath); err != nil {
					return fmt.Errorf("备份旧文件失败: %w", err)
				}
			}

			// 创建新文件
			dst, err := os.Create(exeName)
			if err != nil {
				return err
			}
			defer dst.Close()

			// 复制内容
			if _, err := io.Copy(dst, src); err != nil {
				return err
			}

			return nil
		}
	}

	return fmt.Errorf("在zip文件中找不到mihomo.exe")
}
