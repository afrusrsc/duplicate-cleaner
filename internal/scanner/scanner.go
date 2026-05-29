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
	"path/filepath"
	"runtime"
	"sync"

	"duplicate-cleaner/internal/hasher"
	"duplicate-cleaner/internal/model"
)

// ProgressFunc 是进度回调函数类型。
// current: 已处理文件数; total: 总文件数; path: 当前处理的文件路径。
type ProgressFunc func(current int, total int, path string)

const (
	// sampleThreshold 采样哈希阈值：文件大小超过此值使用采样哈希。
	sampleThreshold = 4 * 1024 * 1024 // 4MB
)

// fileEntry 文件条目：路径和大小。
type fileEntry struct {
	path string
	size int64
}

// hashFunc 计算文件哈希的函数类型。
type hashFunc func(entry fileEntry) (string, error)

// Scan 扫描多个目录，并发计算文件哈希，返回重复文件分组。
func Scan(directories []string, algorithm string, onProgress ProgressFunc) ([]model.DuplicateGroup, error) {
	ctx := context.Background()
	return ScanWithContext(ctx, directories, algorithm, onProgress)
}

// ScanWithContext 带 context 的扫描，支持取消。
func ScanWithContext(ctx context.Context, directories []string, algorithm string, onProgress ProgressFunc) ([]model.DuplicateGroup, error) {
	// 第一阶段：收集所有文件路径和大小
	var allFiles []fileEntry
	for _, dir := range directories {
		entries, err := collectFileEntries(dir)
		if err != nil {
			return nil, fmt.Errorf("扫描目录 %s 失败: %w", dir, err)
		}
		allFiles = append(allFiles, entries...)
	}

	if len(allFiles) == 0 {
		return nil, nil
	}

	// 分离小文件和大文件
	var smallFiles []fileEntry
	var largeFiles []fileEntry
	for _, f := range allFiles {
		if f.size < sampleThreshold {
			smallFiles = append(smallFiles, f)
		} else {
			largeFiles = append(largeFiles, f)
		}
	}

	totalCount := len(allFiles)
	finalMap := make(map[string][]model.FileInfo)
	var mu sync.Mutex

	var processed int
	var progressMu sync.Mutex
	reportProgress := func(path string) {
		if onProgress != nil {
			progressMu.Lock()
			processed++
			current := processed
			progressMu.Unlock()
			onProgress(current, totalCount, path)
		}
	}

	// 处理小文件：直接全量哈希
	if len(smallFiles) > 0 {
		smallMap := hashFiles(ctx, smallFiles, algorithm, reportProgress,
			func(entry fileEntry) (string, error) {
				return hasher.HashFile(entry.path, algorithm)
			})
		mu.Lock()
		for hash, files := range smallMap {
			finalMap[hash] = append(finalMap[hash], files...)
		}
		mu.Unlock()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// 处理大文件：先采样哈希（每个大文件计 1 次进度）
	if len(largeFiles) > 0 {
		sampleMap := hashFiles(ctx, largeFiles, algorithm, reportProgress,
			func(entry fileEntry) (string, error) {
				return hasher.SampleHashFile(entry.path, algorithm, entry.size)
			})
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 对采样匹配的组（≥2 个文件）进行全量哈希验证
		for _, files := range sampleMap {
			if len(files) < 2 {
				continue
			}

			var verifyEntries []fileEntry
			for _, fi := range files {
				verifyEntries = append(verifyEntries, fileEntry{path: fi.Path, size: fi.Size})
			}

			verifyMap := hashFiles(ctx, verifyEntries, algorithm, nil,
				func(entry fileEntry) (string, error) {
					return hasher.HashFile(entry.path, algorithm)
				})
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			mu.Lock()
			for hash, vfiles := range verifyMap {
				finalMap[hash] = append(finalMap[hash], vfiles...)
			}
			mu.Unlock()
		}
	}

	return filterDuplicateGroups(finalMap), nil
}

// collectFileEntries 递归收集目录下所有普通文件的路径和大小。
func collectFileEntries(dir string) ([]fileEntry, error) {
	var entries []fileEntry

	err := filepath.WalkDir(dir, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			entries = append(entries, fileEntry{path: fpath, size: info.Size()})
		}
		return nil
	})

	return entries, err
}

// hashFiles 并发计算文件哈希，按哈希值分组存储。
func hashFiles(ctx context.Context, files []fileEntry, algorithm string, onProgress func(path string), fn hashFunc) map[string][]model.FileInfo {
	fileMap := make(map[string][]model.FileInfo)
	var mu sync.Mutex

	total := len(files)
	numWorkers := runtime.NumCPU()
	if numWorkers > total {
		numWorkers = total
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	type task struct {
		index int
		entry fileEntry
	}
	taskCh := make(chan task, total)
	for i, f := range files {
		taskCh <- task{index: i, entry: f}
	}
	close(taskCh)

	type result struct {
		hash string
		fi   model.FileInfo
		ok   bool
	}
	results := make([]result, total)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				select {
				case <-ctx.Done():
					for range taskCh {
					}
					return
				default:
				}

				hash, err := fn(t.entry)
				if err != nil {
					results[t.index] = result{ok: false}
				} else {
					results[t.index] = result{
						hash: hash,
						fi: model.FileInfo{
							Path: t.entry.path,
							Size: t.entry.size,
							Hash: hash,
						},
						ok: true,
					}
				}

				if onProgress != nil {
					onProgress(t.entry.path)
				}
			}
		}()
	}

	wg.Wait()

	for i := 0; i < total; i++ {
		if results[i].ok {
			r := results[i]
			mu.Lock()
			fileMap[r.hash] = append(fileMap[r.hash], r.fi)
			mu.Unlock()
		}
	}

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
