/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package scanner 提供多目录并发文件扫描与重复检测功能。
package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"duplicate-cleaner/internal/hasher"
	"duplicate-cleaner/internal/model"
)

// ProgressFunc 是进度回调函数类型。
// current: 已处理文件数; total: 总文件数; path: 当前处理的文件路径。
type ProgressFunc func(current int, total int, path string)

// Scan 扫描多个目录，并发计算文件哈希，返回重复文件分组。
// ctx 为 nil 时使用 context.Background()。
func Scan(directories []string, algorithm string, onProgress ProgressFunc) ([]model.DuplicateGroup, error) {
	ctx := context.Background()
	return ScanWithContext(ctx, directories, algorithm, onProgress)
}

// ScanWithContext 带 context 的扫描，支持取消。
func ScanWithContext(ctx context.Context, directories []string, algorithm string, onProgress ProgressFunc) ([]model.DuplicateGroup, error) {
	// 第一阶段：收集所有文件路径
	var allPaths []string
	for _, dir := range directories {
		paths, err := collectFilePaths(dir)
		if err != nil {
			return nil, fmt.Errorf("扫描目录 %s 失败: %w", dir, err)
		}
		allPaths = append(allPaths, paths...)
	}

	if len(allPaths) == 0 {
		return nil, nil
	}

	// 第二阶段：并发计算哈希（支持取消）
	fileMap := concurrentHash(ctx, allPaths, algorithm, onProgress)

	// 如果被取消，返回空结果而不是不全的数据
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 第三阶段：筛选重复组
	return filterDuplicateGroups(fileMap), nil
}

// collectFilePaths 递归收集目录下所有普通文件的路径。
func collectFilePaths(dir string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// 只收集普通文件，跳过目录和符号链接
		if d.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})

	return paths, err
}

// concurrentHash 并发计算文件哈希，按哈希值分组存储。
func concurrentHash(ctx context.Context, paths []string, algorithm string, onProgress ProgressFunc) map[string][]model.FileInfo {
	fileMap := make(map[string][]model.FileInfo)
	var mu sync.Mutex

	total := len(paths)
	numWorkers := runtime.NumCPU()
	if numWorkers > total {
		numWorkers = total
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	// 使用通道分发任务
	pathCh := make(chan string, total)
	for _, p := range paths {
		pathCh <- p
	}
	close(pathCh)

	// 进度计数器
	var processed int
	var progressMu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathCh {
				// 检查是否被取消
				select {
				case <-ctx.Done():
					// 清空剩余任务
					for range pathCh {
					}
					return
				default:
				}

				hash, err := hasher.HashFile(path, algorithm)
				if err != nil {
					continue // 跳过无法读取的文件
				}

				info, err := os.Stat(path)
				if err != nil {
					continue
				}

				fi := model.FileInfo{
					Path: path,
					Size: info.Size(),
					Hash: hash,
				}

				mu.Lock()
				fileMap[hash] = append(fileMap[hash], fi)
				mu.Unlock()

				// 调用进度回调
				if onProgress != nil {
					progressMu.Lock()
					processed++
					current := processed
					progressMu.Unlock()
					onProgress(current, total, path)
				}
			}
		}()
	}

	wg.Wait()
	return fileMap
}

// filterDuplicateGroups 从哈希-文件映射中筛选出有重复的组。
func filterDuplicateGroups(fileMap map[string][]model.FileInfo) []model.DuplicateGroup {
	var groups []model.DuplicateGroup

	for hash, files := range fileMap {
		if len(files) > 1 {
			groups = append(groups, model.DuplicateGroup{
				Hash:  hash,
				Files: files,
			})
		}
	}

	return groups
}
