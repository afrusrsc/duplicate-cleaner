/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package handlers 提供 Web 界面的 HTTP 路由与处理函数。
package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"duplicate-cleaner/internal/deleter"
	"duplicate-cleaner/internal/model"
	"duplicate-cleaner/internal/scanner"
	"duplicate-cleaner/internal/web/views"
)

// ScanState 管理扫描状态，并发安全。
type ScanState struct {
	mu       sync.RWMutex
	scanning bool
	done     bool
	error    string
	current  int
	total    int
	groups   []model.DuplicateGroup
	config   model.Config
}

// Store 全局扫描状态存储。
var Store = &ScanState{}

// Reset 重置扫描状态。
func (s *ScanState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanning = false
	s.done = false
	s.error = ""
	s.current = 0
	s.total = 0
	s.groups = nil
}

// SetDone 标记扫描完成。
func (s *ScanState) SetDone(groups []model.DuplicateGroup, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanning = false
	s.done = true
	if err != nil {
		s.error = err.Error()
	} else {
		s.groups = groups
	}
}

// SetProgress 更新扫描进度。
func (s *ScanState) SetProgress(current, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = current
	s.total = total
}

// GetState 获取当前状态的快照。
func (s *ScanState) GetState() (scanning, done bool, err string, current, total int, groups []model.DuplicateGroup) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanning, s.done, s.error, s.current, s.total, s.groups
}

// HandleIndex 首页处理。
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	views.Index().Render(r.Context(), w)
}

// HandleResults 结果页处理。
func HandleResults(w http.ResponseWriter, r *http.Request) {
	_, done, _, _, _, groups := Store.GetState()
	if !done {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	views.Results(groups, Store.config.DirectDelete).Render(r.Context(), w)
}

// HandleBrowse 目录浏览 API。
func HandleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	// 清理路径，防止目录遍历攻击
	path = filepath.Clean(path)

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"path":  path,
			"dirs":  []string{},
			"error": "无法访问该目录",
		})
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"path":  path,
			"dirs":  []string{},
			"error": err.Error(),
		})
		return
	}

	type dirEntry struct {
		Name      string `json:"name"`
		IsSymlink bool   `json:"isSymlink"`
	}

	var dirs []dirEntry
	for _, entry := range entries {
		name := entry.Name()
		// 跳过隐藏目录
		if strings.HasPrefix(name, ".") {
			continue
		}

		isSymlink := entry.Type()&os.ModeSymlink != 0
		if entry.IsDir() {
			dirs = append(dirs, dirEntry{Name: name, IsSymlink: isSymlink})
		} else if isSymlink {
			// 软链接：检查目标是否为目录
			target, err := os.Stat(filepath.Join(path, name))
			if err == nil && target.IsDir() {
				dirs = append(dirs, dirEntry{Name: name, IsSymlink: true})
			}
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"path": path,
		"dirs": dirs,
	})
}

// HandleScanRequest 扫描请求 API。
func HandleScanRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Directories  []string `json:"directories"`
		Algorithm    string   `json:"algorithm"`
		DirectDelete bool     `json:"directDelete"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求体"}`, http.StatusBadRequest)
		return
	}

	if len(req.Directories) == 0 {
		http.Error(w, `{"error":"请至少指定一个目录"}`, http.StatusBadRequest)
		return
	}

	// 重置状态
	Store.Reset()
	Store.mu.Lock()
	Store.scanning = true
	Store.config = model.Config{
		Directories:  req.Directories,
		Algorithm:    req.Algorithm,
		DirectDelete: req.DirectDelete,
	}
	Store.mu.Unlock()

	// 异步执行扫描
	go func() {
		groups, err := scanner.Scan(req.Directories, req.Algorithm, func(current, total int, path string) {
			Store.SetProgress(current, total)
		})
		Store.SetDone(groups, err)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// HandleProgress 进度查询 API。
func HandleProgress(w http.ResponseWriter, r *http.Request) {
	scanning, done, errStr, current, total, _ := Store.GetState()

	resp := map[string]any{
		"scanning": scanning,
		"done":     done,
		"current":  current,
		"total":    total,
	}
	if errStr != "" {
		resp["error"] = errStr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleDelete 删除文件 API。
func HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Paths []string `json:"paths"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求体"}`, http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		http.Error(w, `{"error":"请选择要删除的文件"}`, http.StatusBadRequest)
		return
	}

	_, _, _, _, _, groups := Store.GetState()
	_ = groups
	directDelete := Store.config.DirectDelete

	results := deleter.DeleteMultiple(req.Paths, directDelete)

	successCount := 0
	failCount := 0
	deletedPaths := make(map[string]bool)
	for _, r := range results {
		if r.Error == nil {
			successCount++
			deletedPaths[r.Path] = true
		} else {
			failCount++
		}
	}

	// 从 groups 中移除已删除的文件，清理只剩 <=1 个文件的组
	if successCount > 0 {
		Store.mu.Lock()
		var filtered []model.DuplicateGroup
		for _, g := range Store.groups {
			var remaining []model.FileInfo
			for _, f := range g.Files {
				if !deletedPaths[f.Path] {
					remaining = append(remaining, f)
				}
			}
			if len(remaining) >= 2 {
				filtered = append(filtered, model.DuplicateGroup{
					Hash:  g.Hash,
					Files: remaining,
				})
			}
		}
		Store.groups = filtered
		Store.mu.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": successCount,
		"fail":    failCount,
	})
}

// NewRouter 创建并配置 HTTP 路由。
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// 页面路由
	mux.HandleFunc("/", HandleIndex)
	mux.HandleFunc("/results", HandleResults)

	// API 路由
	mux.HandleFunc("/api/scan", HandleScanRequest)
	mux.HandleFunc("/api/progress", HandleProgress)
	mux.HandleFunc("/api/delete", HandleDelete)
	mux.HandleFunc("/api/browse", HandleBrowse)

	return mux
}
