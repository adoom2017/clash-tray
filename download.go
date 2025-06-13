package main

import (
	"archive/zip"
	"clash-tray/log"
	"encoding/json"
	"errors"
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

// 定义常量
const (
	// GitHub API地址
	mihomoRepoAPI = "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
	// 临时文件夹
	tempDir = "./temp"
	// 下载超时时间
	downloadTimeout = 60 * time.Second
	// 用于HTTP请求的User-Agent
	userAgent = "Clash-Tray"
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

// 获取目标平台和架构的资源名称模式
func getAssetPattern(version string) string {
	// 根据当前系统运行时确定架构和平台
	arch := "amd64" // 可根据runtime.GOARCH动态获取
	platform := "windows"
	return fmt.Sprintf("mihomo-%s-%s-compatible-%s.zip", platform, arch, version)
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
	re := regexp.MustCompile(`v\d+(\.\d+)+`)
	versionMatch := re.FindString(string(output))

	if versionMatch == "" {
		return "", fmt.Errorf("无法从输出中提取版本号: %s", string(output))
	}

	return versionMatch, nil
}

// 创建HTTP客户端
func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: downloadTimeout,
	}
}

// 获取最新版本信息
func getLatestReleaseInfo(client *http.Client) (*GithubRelease, error) {
	req, err := http.NewRequest("GET", mihomoRepoAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求返回非200状态码: %d", resp.StatusCode)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析API响应JSON失败: %w", err)
	}

	return &release, nil
}

// 查找匹配当前系统的资源
func findMatchingAsset(release *GithubRelease) (string, string, error) {
	assetPattern := getAssetPattern(release.TagName)

	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, strings.ToLower(assetPattern)) && strings.HasSuffix(name, ".zip") {
			return asset.BrowserDownloadURL, asset.Name, nil
		}
	}

	return "", "", fmt.Errorf("找不到适合当前系统的下载资源")
}

// 确保临时目录存在
func ensureTempDir() error {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	return nil
}

// 下载文件到指定路径
func downloadFile(url, filePath string, client *http.Client) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载请求返回非200状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// 从zip文件中提取可执行文件
func extractExecutable(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开ZIP文件失败: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			// 打开文件
			src, err := f.Open()
			if err != nil {
				return fmt.Errorf("打开ZIP中的文件失败: %w", err)
			}

			// 备份现有文件
			if _, err := os.Stat(exeName); err == nil {
				backupPath := exeName + ".bak"
				if err := os.Rename(exeName, backupPath); err != nil {
					src.Close()
					return fmt.Errorf("备份原文件失败: %w", err)
				}
				log.Infoln("已备份原文件为: " + backupPath)
			}

			// 创建新文件
			dst, err := os.Create(exeName)
			if err != nil {
				src.Close()
				return fmt.Errorf("创建新文件失败: %w", err)
			}

			// 复制内容
			if _, err := io.Copy(dst, src); err != nil {
				src.Close()
				dst.Close()
				return fmt.Errorf("复制文件内容失败: %w", err)
			}

			src.Close()
			dst.Close()

			log.Infoln("已成功提取: " + f.Name + " → " + exeName)
			return nil
		}
	}

	return fmt.Errorf("在ZIP文件中找不到可执行文件(.exe)")
}

// 清理临时文件
func cleanupTempFiles() {
	if err := os.RemoveAll(tempDir); err != nil {
		log.Warnln("清理临时文件失败: %v", err)
	}
}

// 检查是否需要更新
func needUpdate(currentVersion, latestVersion string) (bool, string) {
	if currentVersion == latestVersion {
		return false, fmt.Sprintf("当前已是最新版本 %s，无需更新", currentVersion)
	}
	return true, fmt.Sprintf("发现新版本: 当前版本 %s, 最新版本 %s", currentVersion, latestVersion)
}

// 下载Mihomo最新版本 - 主函数
func DownloadLatestMihomo() error {
	// 获取当前版本
	currentVersion, err := getCurrentVersion()
	if err != nil {
		log.Warnln("获取当前版本失败: %v", err)
		// 继续更新，因为可能是首次安装
	}

	// 创建HTTP客户端
	client := createHTTPClient()

	// 获取最新版本信息
	log.Infoln("正在获取Mihomo最新版本信息...")
	release, err := getLatestReleaseInfo(client)
	if err != nil {
		return fmt.Errorf("获取最新版本信息失败: %w", err)
	}

	log.Infoln("GitHub最新版本: " + release.TagName)

	// 检查是否需要更新
	if currentVersion != "" {
		needToUpdate, message := needUpdate(currentVersion, release.TagName)
		if !needToUpdate {
			log.Infoln(message)
			return errors.New(message)
		}
		log.Infoln(message)
	}

	// 查找匹配当前系统的资源
	assetURL, assetName, err := findMatchingAsset(release)
	if err != nil {
		return err
	}

	log.Infoln("准备下载: " + assetName)

	// 确保临时目录存在
	if err := ensureTempDir(); err != nil {
		return err
	}

	// 下载文件
	zipPath := filepath.Join(tempDir, assetName)
	if err := downloadFile(assetURL, zipPath, client); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	log.Infoln("下载完成，开始解压...")

	// 解压文件
	if err := extractExecutable(zipPath); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	log.Infoln("Mihomo更新完成！")

	// 清理临时文件
	cleanupTempFiles()

	return nil
}
