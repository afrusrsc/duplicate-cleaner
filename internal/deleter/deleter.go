/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package deleter 提供文件删除功能，支持直接删除和移到回收站。
package deleter

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// DeleteResult 记录单个文件的删除结果。
type DeleteResult struct {
	Path  string // 文件路径
	Error error  // 删除失败时的错误信息
}

// Delete 删除指定文件。
// directDelete 为 true 时直接删除，为 false 时移到回收站。
func Delete(path string, directDelete bool) error {
	if directDelete {
		return os.Remove(path)
	}
	return moveToTrash(path)
}

// DeleteMultiple 批量删除文件，返回每个文件的删除结果。
func DeleteMultiple(paths []string, directDelete bool) []DeleteResult {
	results := make([]DeleteResult, len(paths))
	for i, path := range paths {
		err := Delete(path, directDelete)
		results[i] = DeleteResult{
			Path:  path,
			Error: err,
		}
	}
	return results
}

// moveToTrash 将文件移到回收站（跨平台）。
func moveToTrash(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return moveToTrashDarwin(path)
	case "windows":
		return moveToTrashWindows(path)
	default: // linux 及其他 unix
		return moveToTrashLinux(path)
	}
}

// --- Linux: FreeDesktop.org Trash 规范 ---

// trashBasePathLinux 返回 FreeDesktop.org 标准的回收站根目录。
func trashBasePathLinux() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取 HOME 目录失败: %w", err)
	}
	return filepath.Join(home, ".local", "share", "Trash"), nil
}

// ensureTrashDirLinux 确保回收站目录存在。
func ensureTrashDirLinux() (string, error) {
	base, err := trashBasePathLinux()
	if err != nil {
		return "", err
	}
	filesDir := filepath.Join(base, "files")
	infoDir := filepath.Join(base, "info")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return "", fmt.Errorf("创建回收站 files 目录失败: %w", err)
	}
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return "", fmt.Errorf("创建回收站 info 目录失败: %w", err)
	}
	return base, nil
}

func moveToTrashLinux(path string) error {
	base, err := ensureTrashDirLinux()
	if err != nil {
		return err
	}

	fileName := filepath.Base(path)
	trashedPath := filepath.Join(base, "files", fileName)
	infoPath := filepath.Join(base, "info", fileName+".trashinfo")

	// 如果回收站中已存在同名文件，添加时间戳后缀
	if _, err := os.Stat(trashedPath); err == nil {
		suffix := fmt.Sprintf("_%d", time.Now().UnixNano())
		trashedPath = filepath.Join(base, "files", fileName+suffix)
		infoPath = filepath.Join(base, "info", fileName+suffix+".trashinfo")
	}

	// 写入 .trashinfo 文件
	now := time.Now().Format(time.RFC3339)
	infoContent := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n", path, now)
	if err := os.WriteFile(infoPath, []byte(infoContent), 0644); err != nil {
		return fmt.Errorf("写入回收站信息文件失败: %w", err)
	}

	// 移动文件到回收站（支持跨设备）
	if err := moveFile(path, trashedPath); err != nil {
		os.Remove(infoPath)
		return fmt.Errorf("移动文件到回收站失败: %w", err)
	}

	return nil
}

// --- macOS: 移到 ~/.Trash ---

func moveToTrashDarwin(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取 HOME 目录失败: %w", err)
	}

	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		return fmt.Errorf("创建回收站目录失败: %w", err)
	}

	fileName := filepath.Base(path)
	trashedPath := filepath.Join(trashDir, fileName)

	// 如果回收站中已存在同名文件，添加时间戳后缀
	if _, err := os.Stat(trashedPath); err == nil {
		suffix := time.Now().Format("_2006-01-02-15.04.05")
		trashedPath = filepath.Join(trashDir, fileName+suffix)
	}

	if err := moveFile(path, trashedPath); err != nil {
		return fmt.Errorf("移动文件到回收站失败: %w", err)
	}

	return nil
}

// --- Windows: 调用 PowerShell 移到回收站 ---

func moveToTrashWindows(path string) error {
	// 使用 .NET 的 Microsoft.VisualBasic.FileIO.FileSystem 支持移到回收站
	script := fmt.Sprintf(
		`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile('%s', 'OnlyErrorDialogs', 'SendToRecycleBin')`,
		path,
	)

	// 先检查是文件还是目录
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("访问文件失败: %w", err)
	}

	if info.IsDir() {
		script = fmt.Sprintf(
			`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteDirectory('%s', 'OnlyErrorDialogs', 'SendToRecycleBin')`,
			path,
		)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("移动到回收站失败: %w (%s)", err, string(output))
	}

	return nil
}

// --- 通用工具函数 ---

// copyFile 复制文件内容到目标路径。
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// 保留原文件权限
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// moveFile 安全移动文件，跨设备时使用复制+删除。
func moveFile(src, dst string) error {
	// 先尝试直接重命名（同设备下最快）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 跨设备：复制后删除
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}
	return os.Remove(src)
}
