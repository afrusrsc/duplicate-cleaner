/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// duplicate 文件去重
package duplicate

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
)

// 单个文件信息
type FileInfo struct {
	Path string
	Size int64
	Hash string
}

type FileInfos []FileInfo

type DupList map[string]FileInfos

// List 获取重复文件的列表
func List(dirs []string, hashName string, n int) (DupList, error) {
	fs, err := walkDirs(dirs)
	if err != nil {
		return nil, err
	}
	fs = groupBySize(fs)
	err = calcHashs(fs, hashName, n)
	lst := groupByHash(fs)
	return lst, err
}

// Clean 删除重复的文件
func Clean(files []string) (int, error) {
	if len(files) == 0 {
		return 0, nil
	}
	n := 0
	errs := []error{}
	bar := progressbar.Default(int64(len(files)), "清理文件")
	defer bar.Close()
	for _, file := range files {
		err := os.Remove(file)
		bar.Add(1)
		if err != nil {
			errs = append(errs, fmt.Errorf("文件%s清理失败: %v", file, err))
		} else {
			n += 1
		}
	}
	return n, errors.Join(errs...)
}

// walkDirs 遍历指定目录获取文件信息
func walkDirs(dirs []string) ([]*FileInfo, error) {
	if len(dirs) == 0 {
		return nil, errors.Join(errors.New("目录未指定"))
	}
	var files []*FileInfo
	bar := progressbar.Default(-1, "遍历文件")
	defer bar.Close()
	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			log.Printf("无法获取绝对路径: %v", err)
			continue
		}
		err = filepath.Walk(absDir, func(path string, info fs.FileInfo, err error) error {
			bar.Add(1)
			// 跳过无法访问的目录
			if err != nil {
				return filepath.SkipDir
			}
			// 跳过代码库
			if info.IsDir() && (strings.EqualFold(filepath.Base(path), ".git") || strings.EqualFold(filepath.Base(path), ".svn")) {
				return filepath.SkipDir
			}
			//跳过特殊文件
			if !info.Mode().IsRegular() {
				return nil
			}
			if info.Size() > 0 {
				files = append(files, &FileInfo{
					Path: path,
					Size: info.Size(),
				})
			}
			return nil
		})
		if err != nil {
			return nil, errors.Join(err)
		}
	}
	return files, nil
}

// newHash 创建对应的Hash实例
func newHash(hashName string) hash.Hash {
	var h hash.Hash
	switch strings.ToLower(hashName) {
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		h = md5.New()
	}
	return h
}

// calcHash 计算文件的Hash值
func calcHash(file string, h hash.Hash) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", errors.Join(err)
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", errors.Join(err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// calcHashs 并行计算多个文件的Hash值
func calcHashs(files []*FileInfo, hashName string, n int) error {
	if len(files) == 0 {
		return nil
	}
	g := sync.WaitGroup{}
	c := make(chan struct{}, n)
	m := sync.Mutex{}
	errs := []error{}
	bar := progressbar.Default(int64(len(files)), "计算Hash值")
	defer bar.Close()
	for _, file := range files {
		g.Add(1)
		go func(f *FileInfo) {
			defer g.Done()
			c <- struct{}{}
			defer func() { <-c }()
			// hash.Hash接口不是并发安全的，要在协程内实例化
			h := newHash(hashName)
			hashValue, err := calcHash(f.Path, h)
			bar.Add(1)
			if err != nil {
				m.Lock()
				errs = append(errs, fmt.Errorf("计算文件 %s 的Hash值失败: %v", f.Path, err))
				m.Unlock()
			} else {
				f.Hash = hashValue
			}
		}(file)
	}
	g.Wait()
	return errors.Join(errs...)
}

// groupByHash 按Hash值进行分组，并删除Hash值唯一的记录
func groupByHash(files []*FileInfo) DupList {
	if len(files) == 0 {
		return nil
	}
	group := DupList{}
	counts := map[string]int{}
	bar := progressbar.Default(-1, "按Hash值分组")
	defer bar.Close()
	for _, file := range files {
		if file.Hash != "" {
			group[file.Hash] = append(group[file.Hash], *file)
			counts[file.Hash] += 1
			bar.Add(1)
		}
	}
	bar.Clear()
	bar.Describe("剔除孤立组")
	for k, v := range counts {
		if v == 1 {
			delete(group, k)
			bar.Add(1)
		}
	}
	return group
}

// groupBySize 按大小进行分组，并删除大小唯一的记录
func groupBySize(files []*FileInfo) []*FileInfo {
	if len(files) == 0 {
		return nil
	}
	group := map[int64][]*FileInfo{}
	newFiles := []*FileInfo{}
	bar := progressbar.Default(-1, "按大小分组")
	defer bar.Close()
	for _, file := range files {
		group[file.Size] = append(group[file.Size], file)
		bar.Add(1)
	}
	bar.Clear()
	bar.Describe("剔除孤立组")
	for k, v := range group {
		if len(v) == 1 {
			delete(group, k)
			bar.Add(1)
		} else {
			newFiles = append(newFiles, v...)
		}
	}
	return newFiles
}
