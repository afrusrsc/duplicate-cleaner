/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

package hasher

import (
	"os"
	"path/filepath"
	"testing"
)

// createTempFile 创建一个临时文件并写入指定内容，返回文件路径。
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	return path
}

func TestHashFile_XXHash(t *testing.T) {
	path := createTempFile(t, "hello world")
	got, err := HashFile(path, "xxhash")
	if err != nil {
		t.Fatalf("HashFile 失败: %v", err)
	}
	// "hello world" 的 XXHash64 已知值
	if len(got) != 16 {
		t.Errorf("XXHash 期望 16 位十六进制字符串，得到 %d 位: %s", len(got), got)
	}
}

func TestHashFile_SHA256(t *testing.T) {
	path := createTempFile(t, "hello world")
	got, err := HashFile(path, "sha256")
	if err != nil {
		t.Fatalf("HashFile 失败: %v", err)
	}
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if got != want {
		t.Errorf("SHA256 不匹配\ngot:  %s\nwant: %s", got, want)
	}
}

func TestHashFile_SHA512(t *testing.T) {
	path := createTempFile(t, "hello world")
	got, err := HashFile(path, "sha512")
	if err != nil {
		t.Fatalf("HashFile 失败: %v", err)
	}
	want := "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f"
	if got != want {
		t.Errorf("SHA512 不匹配\ngot:  %s\nwant: %s", got, want)
	}
}

func TestHashFile_UnsupportedAlgorithm(t *testing.T) {
	path := createTempFile(t, "hello world")
	_, err := HashFile(path, "unknown")
	if err == nil {
		t.Error("期望返回错误，但得到了 nil")
	}
}

func TestHashFile_FileNotFound(t *testing.T) {
	_, err := HashFile("/nonexistent/file/path", "sha256")
	if err == nil {
		t.Error("期望返回错误，但得到了 nil")
	}
}

func TestHashFile_EmptyFile(t *testing.T) {
	path := createTempFile(t, "")
	got, err := HashFile(path, "sha256")
	if err != nil {
		t.Fatalf("HashFile 失败: %v", err)
	}
	// 空文件的 SHA256
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("空文件 SHA256 不匹配\ngot:  %s\nwant: %s", got, want)
	}
}

func TestValidAlgorithm(t *testing.T) {
	tests := []struct {
		algo string
		want bool
	}{
		{"xxhash", true},
		{"sha256", true},
		{"sha512", true},
		{"md5", false},
		{"sha1", false},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		got := ValidAlgorithm(tt.algo)
		if got != tt.want {
			t.Errorf("ValidAlgorithm(%q) = %v, want %v", tt.algo, got, tt.want)
		}
	}
}
