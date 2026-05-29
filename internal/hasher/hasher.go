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

// HashFile 计算指定文件路径的全量哈希值，返回十六进制编码的哈希字符串。
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

// SampleHashFile 对大文件进行采样哈希：读取 5 个 4KB 采样块计算哈希。
// fileSize 是文件大小（传入避免重复 Stat）。
// 采样位置：开头、1/4、1/2、3/4、结尾。
func SampleHashFile(path string, algo string, fileSize int64) (string, error) {
	h, err := newHasher(algo)
	if err != nil {
		return "", err
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败 %s: %w", path, err)
	}
	defer file.Close()

	const sampleSize = int64(4096)

	positions := [5]int64{
		0,
		fileSize / 4,
		fileSize / 2,
		3 * fileSize / 4,
		fileSize - sampleSize,
	}
	// 确保最后一个采样位置不小于 0
	if positions[4] < 0 {
		positions[4] = 0
	}

	// 文件小于 4KB 时，确保不重复读
	var readRanges [][2]int64

	for _, offset := range positions {
		end := offset + sampleSize
		if end > fileSize {
			end = fileSize
		}
		if offset >= end {
			continue
		}

		// 合并相邻或重叠的读取范围
		if n := len(readRanges); n > 0 && readRanges[n-1][1] >= offset {
			if end > readRanges[n-1][1] {
				readRanges[n-1][1] = end
			}
		} else {
			readRanges = append(readRanges, [2]int64{offset, end})
		}
	}

	buf := make([]byte, sampleSize)
	for _, r := range readRanges {
		file.Seek(r[0], io.SeekStart)
		_, err := io.ReadFull(file, buf[:r[1]-r[0]])
		if err != nil {
			return "", fmt.Errorf("读取采样块失败 %s: %w", path, err)
		}
		h.Write(buf[:r[1]-r[0]])
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
