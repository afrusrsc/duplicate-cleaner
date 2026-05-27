/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

package deleter

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "testfile")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}
	return path
}

func TestDelete_DirectDelete(t *testing.T) {
	path := createTestFile(t, "hello")

	// 确认文件存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("测试文件应存在")
	}

	err := Delete(path, true)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	// 确认文件已被删除
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("文件应已被直接删除")
	}
}

func TestDelete_DirectDelete_NotFound(t *testing.T) {
	err := Delete("/nonexistent/file/path", true)
	if err == nil {
		t.Error("期望返回错误，但得到了 nil")
	}
}

func TestDelete_ToTrash(t *testing.T) {
	path := createTestFile(t, "hello world")

	// 确认文件存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("测试文件应存在")
	}

	err := Delete(path, false)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}

	// 确认原文件已被移走
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("原文件应已被移到回收站")
	}
}

func TestDelete_ToTrash_NotFound(t *testing.T) {
	err := Delete("/nonexistent/file/path", false)
	if err == nil {
		t.Error("期望返回错误，但得到了 nil")
	}
}

func TestDeleteMultiple(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, string(rune('a'+i))+".txt")
		if err := os.WriteFile(paths[i], []byte("content"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	results := DeleteMultiple(paths, true)

	if len(results) != 3 {
		t.Fatalf("期望 3 个结果，得到 %d 个", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("文件 %d 删除失败: %v", i, r.Error)
		}
		if _, err := os.Stat(r.Path); !os.IsNotExist(err) {
			t.Errorf("文件 %s 应已被删除", r.Path)
		}
	}
}

func TestDeleteMultiple_MixedResults(t *testing.T) {
	dir := t.TempDir()
	validPath := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(validPath, []byte("content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	paths := []string{validPath, "/nonexistent/file"}

	results := DeleteMultiple(paths, true)

	// 第一个应成功，第二个应失败
	if results[0].Error != nil {
		t.Errorf("有效文件删除应成功: %v", results[0].Error)
	}
	if results[1].Error == nil {
		t.Error("无效文件删除应失败")
	}
}

func TestEnsureTrashDirLinux(t *testing.T) {
	// 使用临时目录作为 HOME
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	base, err := ensureTrashDirLinux()
	if err != nil {
		t.Fatalf("ensureTrashDirLinux 失败: %v", err)
	}

	trashFiles := filepath.Join(base, "files")
	trashInfo := filepath.Join(base, "info")

	if _, err := os.Stat(trashFiles); os.IsNotExist(err) {
		t.Error("回收站 files 目录应存在")
	}
	if _, err := os.Stat(trashInfo); os.IsNotExist(err) {
		t.Error("回收站 info 目录应存在")
	}
}
