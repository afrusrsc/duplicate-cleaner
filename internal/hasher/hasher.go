/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package hasher 提供文件哈希计算功能，支持 xxhash、sha256、sha512。
package hasher

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"

	xxhash "github.com/cespare/xxhash/v2"
)

// ValidAlgorithm 检查算法名称是否受支持。
func ValidAlgorithm(algo string) bool {
	switch algo {
	case "xxhash", "sha256", "sha512":
		return true
	default:
		return false
	}
}

// newHasher 根据算法名称创建对应的 hash.Hash 实例。
func newHasher(algo string) (hash.Hash, error) {
	switch algo {
	case "xxhash":
		return xxhash.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("不支持的哈希算法: %s（可选: xxhash, sha256, sha512）", algo)
	}
}

// HashFile 计算指定文件路径的哈希值，返回十六进制编码的哈希字符串。
func HashFile(path string, algo string) (string, error) {
	h, err := newHasher(algo)
	if err != nil {
		return "", err
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败 %s: %w", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("读取文件失败 %s: %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
