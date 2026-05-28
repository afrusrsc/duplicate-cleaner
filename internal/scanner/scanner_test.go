/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

package scanner

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"duplicate-cleaner/internal/model"
)

// setupTestFiles 创建测试目录结构：
//
//	dir1/
//	  a.txt  ("hello")
//	  b.txt  ("world")
//	  c.txt  ("hello")   // 与 a.txt 重复
//	dir2/
//	  d.txt  ("hello")   // 与 a.txt 跨目录重复
//	  e.txt  ("unique")
func setupTestFiles(t *testing.T) (dir1, dir2 string) {
	t.Helper()

	dir1 = t.TempDir()
	dir2 = t.TempDir()

	files := map[string]string{
		filepath.Join(dir1, "a.txt"): "hello",
		filepath.Join(dir1, "b.txt"): "world",
		filepath.Join(dir1, "c.txt"): "hello",
		filepath.Join(dir2, "d.txt"): "hello",
		filepath.Join(dir2, "e.txt"): "unique",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("创建测试文件失败 %s: %v", path, err)
		}
	}

	return dir1, dir2
}

func TestScan_SingleDirectory(t *testing.T) {
	dir1, _ := setupTestFiles(t)

	groups, err := Scan([]string{dir1}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	// dir1 中只有一组重复: a.txt 和 c.txt (内容都是 "hello")
	if len(groups) != 1 {
		t.Fatalf("期望 1 组重复，得到 %d 组", len(groups))
	}

	if len(groups[0].Files) != 2 {
		t.Errorf("期望重复组有 2 个文件，得到 %d 个", len(groups[0].Files))
	}

	// b.txt 内容唯一，不应出现在任何重复组中
	for _, g := range groups {
		for _, f := range g.Files {
			if filepath.Base(f.Path) == "b.txt" {
				t.Error("b.txt 不应出现在重复组中")
			}
		}
	}
}

func TestScan_MultipleDirectories(t *testing.T) {
	dir1, dir2 := setupTestFiles(t)

	groups, err := Scan([]string{dir1, dir2}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	// 跨目录: a.txt, c.txt, d.txt 内容都是 "hello"
	if len(groups) != 1 {
		t.Fatalf("期望 1 组重复，得到 %d 组", len(groups))
	}

	if len(groups[0].Files) != 3 {
		t.Errorf("期望重复组有 3 个文件（跨目录），得到 %d 个", len(groups[0].Files))
	}

	// 验证包含了不同目录的文件
	foundDir2 := false
	for _, f := range groups[0].Files {
		if filepath.Base(f.Path) == "d.txt" {
			foundDir2 = true
		}
	}
	if !foundDir2 {
		t.Error("重复组应包含 dir2 中的 d.txt")
	}
}

func TestScan_NoDuplicates(t *testing.T) {
	dir := t.TempDir()

	// 创建内容各不相同的文件
	for i, content := range []string{"aaa", "bbb", "ccc"} {
		path := filepath.Join(dir, string(rune('a'+i))+".txt")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	groups, err := Scan([]string{dir}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("期望没有重复组，得到 %d 组", len(groups))
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	groups, err := Scan([]string{dir}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(groups) != 0 {
		t.Errorf("期望没有重复组，得到 %d 组", len(groups))
	}
}

func TestScan_ProgressCallback(t *testing.T) {
	dir1, _ := setupTestFiles(t)

	var progressCalls atomic.Int32
	onProgress := func(current, total int, path string) {
		progressCalls.Add(1)
	}

	_, err := Scan([]string{dir1}, "xxhash", onProgress)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	// dir1 有 3 个文件，应收到 3 次进度回调
	if got := progressCalls.Load(); got != 3 {
		t.Errorf("期望 3 次进度回调，得到 %d 次", got)
	}
}

func TestScan_InvalidDirectory(t *testing.T) {
	_, err := Scan([]string{"/nonexistent/path"}, "xxhash", nil)
	if err == nil {
		t.Error("期望返回错误，但得到了 nil")
	}
}

func TestScan_SubDirectories(t *testing.T) {
	dir := t.TempDir()

	// 创建嵌套目录结构
	subDir := filepath.Join(dir, "sub1", "sub2")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}

	// 在根目录和子目录放相同内容的文件
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("same"), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.txt"), []byte("same"), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	groups, err := Scan([]string{dir}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("期望 1 组重复，得到 %d 组", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Errorf("期望 2 个文件，得到 %d 个", len(groups[0].Files))
	}
}

func TestScan_FileInfoIntegrity(t *testing.T) {
	dir1, _ := setupTestFiles(t)

	groups, err := Scan([]string{dir1}, "xxhash", nil)
	if err != nil {
		t.Fatalf("Scan 失败: %v", err)
	}

	if len(groups) == 0 {
		t.Fatal("期望有重复组")
	}

	for _, g := range groups {
		if g.Hash == "" {
			t.Error("哈希值不应为空")
		}
		for _, f := range g.Files {
			if f.Path == "" {
				t.Error("文件路径不应为空")
			}
			if f.Size < 0 {
				t.Errorf("文件大小不应为负数: %d", f.Size)
			}
			if f.Hash != g.Hash {
				t.Errorf("文件哈希 %q 与组哈希 %q 不匹配", f.Hash, g.Hash)
			}
		}
	}
}

func TestScan_DifferentAlgorithms(t *testing.T) {
	dir1, _ := setupTestFiles(t)

	for _, algo := range []string{"xxhash", "sha256", "sha512"} {
		t.Run(algo, func(t *testing.T) {
			groups, err := Scan([]string{dir1}, algo, nil)
			if err != nil {
				t.Fatalf("Scan(%s) 失败: %v", algo, err)
			}
			if len(groups) != 1 {
				t.Errorf("算法 %s: 期望 1 组重复，得到 %d 组", algo, len(groups))
			}
		})
	}
}

func TestFilterDuplicateGroups(t *testing.T) {
	// 模拟扫描结果：有些文件只有一个，有些有多个
	fileMap := map[string][]model.FileInfo{
		"hash_a": {
			{Path: "/a1", Size: 10, Hash: "hash_a"},
			{Path: "/a2", Size: 10, Hash: "hash_a"},
		},
		"hash_b": {
			{Path: "/b1", Size: 20, Hash: "hash_b"},
		},
		"hash_c": {
			{Path: "/c1", Size: 30, Hash: "hash_c"},
			{Path: "/c2", Size: 30, Hash: "hash_c"},
			{Path: "/c3", Size: 30, Hash: "hash_c"},
		},
	}

	groups := filterDuplicateGroups(fileMap)
	if len(groups) != 2 {
		t.Fatalf("期望 2 组重复，得到 %d 组", len(groups))
	}

	// hash_a 组应有 2 个文件
	// hash_c 组应有 3 个文件
	totalFiles := 0
	for _, g := range groups {
		totalFiles += len(g.Files)
	}
	if totalFiles != 5 {
		t.Errorf("期望总共 5 个文件，得到 %d 个", totalFiles)
	}
}
