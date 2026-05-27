/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package tui 提供 Bubble Tea 终端界面。
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"duplicate-cleaner/internal/deleter"
	"duplicate-cleaner/internal/model"
	"duplicate-cleaner/internal/scanner"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 界面状态
type state int

const (
	stateInput    state = iota // 输入配置
	stateBrowse                // 目录浏览
	stateScanning              // 扫描中
	stateResults               // 显示结果
	stateConfirm               // 确认删除
	stateDone                  // 完成
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			MarginBottom(1)

	groupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2563EB")).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DC2626"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#059669"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DC2626"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1)

	progressBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C3AED"))

	dirSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7C3AED"))
)

// dirEntry 目录条目
type dirEntry struct {
	Name      string
	IsSymlink bool
}

// scanProgressMsg 扫描进度消息
type scanProgressMsg struct {
	current int
	total   int
	path    string
}

// scanDoneMsg 扫描完成消息
type scanDoneMsg struct {
	groups []model.DuplicateGroup
	err    error
}

// deleteDoneMsg 删除完成消息
type deleteDoneMsg struct {
	results []deleter.DeleteResult
}

// MainModel 是 TUI 的主模型
type MainModel struct {
	state    state
	config   model.Config
	err      error
	viewport viewport.Model

	// 输入状态
	dirInput     textinput.Model
	algoChoices  []string
	algoIndex    int
	directDelete bool

	// 目录浏览状态
	browsePath   string
	browseDirs   []dirEntry
	browseCursor int

	// 已选目录
	directories []string
	dirCursor   int // 已选目录列表光标（-1 表示不在列表中）

	// 扫描状态
	progressCurrent int
	progressTotal   int
	progressPath    string
	progressCh      chan scanProgressMsg

	// 结果状态
	groups     []model.DuplicateGroup
	cursor     int
	selected   map[string]bool
	totalFiles int

	// 确认状态
	deleteResults []deleter.DeleteResult

	// 窗口尺寸
	width  int
	height int
}

// NewMainModel 创建 TUI 主模型
func NewMainModel(config model.Config) MainModel {
	dirInput := textinput.New()
	dirInput.Placeholder = "输入目录路径（多个路径用逗号分隔）"
	dirInput.Focus()
	dirInput.CharLimit = 500
	dirInput.Width = 50

	if len(config.Directories) > 0 {
		dirInput.SetValue(strings.Join(config.Directories, ","))
	}

	algoChoices := []string{"md5", "sha1", "sha256", "sha512"}
	algoIndex := 0
	for i, a := range algoChoices {
		if a == config.Algorithm {
			algoIndex = i
			break
		}
	}

	m := MainModel{
		state:        stateInput,
		config:       config,
		dirInput:     dirInput,
		algoChoices:  algoChoices,
		algoIndex:    algoIndex,
		directDelete: config.DirectDelete,
		selected:     make(map[string]bool),
		directories:  []string{},
	}
	m.browsePath = "/"
	m.loadBrowseDirs("/")
	return m
}

func (m MainModel) Init() tea.Cmd {
	return textinput.Blink
}

// waitForProgress 等待下一个进度消息
func waitForProgress(ch chan scanProgressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8
		return m, nil

	case tea.KeyMsg:
		// 全局快捷键
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state != stateInput && m.state != stateBrowse {
				return m, tea.Quit
			}
		}

	case scanProgressMsg:
		if m.state == stateScanning {
			m.progressCurrent = msg.current
			m.progressTotal = msg.total
			m.progressPath = filepath.Base(msg.path)
			return m, waitForProgress(m.progressCh)
		}
		return m, nil

	case scanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = stateInput
			return m, nil
		}
		m.groups = msg.groups
		m.state = stateResults
		m.selected = make(map[string]bool)
		m.totalFiles = 0
		for _, g := range m.groups {
			m.totalFiles += len(g.Files)
		}
		m.cursor = 0
		m.buildResultsView()
		m.viewport.GotoTop()
		return m, nil

	case deleteDoneMsg:
		m.deleteResults = msg.results
		deletedPaths := make(map[string]bool)
		for _, r := range msg.results {
			if r.Error == nil {
				deletedPaths[r.Path] = true
			}
		}
		m.removeDeletedFiles(deletedPaths)
		m.selected = make(map[string]bool)
		m.cursor = 0
		m.state = stateResults
		m.buildResultsView()
		m.viewport.GotoTop()
		return m, nil
	}

	// 根据状态处理
	switch m.state {
	case stateInput:
		return m.updateInput(msg)
	case stateBrowse:
		return m.updateBrowse(msg)
	case stateScanning:
		return m.updateScanning(msg)
	case stateResults:
		return m.updateResults(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	case stateDone:
		return m.updateDone(msg)
	}

	return m, nil
}

func (m MainModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// 合并 directories 和文本输入
			allDirs := make([]string, 0, len(m.directories)+1)
			allDirs = append(allDirs, m.directories...)
			input := strings.TrimSpace(m.dirInput.Value())
			if input != "" {
				dirs := strings.Split(input, ",")
				for _, d := range dirs {
					d = strings.TrimSpace(d)
					if d != "" {
						allDirs = append(allDirs, d)
					}
				}
			}

			if len(allDirs) == 0 {
				m.err = fmt.Errorf("请输入至少一个目录路径")
				return m, nil
			}
			m.err = nil
			m.config.Directories = allDirs
			m.config.Algorithm = m.algoChoices[m.algoIndex]
			m.config.DirectDelete = m.directDelete

			m.state = stateScanning
			ch := make(chan scanProgressMsg, 64)
			m.progressCh = ch
			return m, m.startScanWithCh(ch)

		case "tab":
			m.algoIndex = (m.algoIndex + 1) % len(m.algoChoices)
			return m, nil

		case "d":
			m.directDelete = !m.directDelete
			return m, nil

		case "delete", "x":
			if m.dirCursor >= 0 && m.dirCursor < len(m.directories) {
				m.directories = append(m.directories[:m.dirCursor], m.directories[m.dirCursor+1:]...)
				if m.dirCursor >= len(m.directories) {
					m.dirCursor = len(m.directories) - 1
				}
				if len(m.directories) == 0 {
					m.dirCursor = -1
				}
			}
			return m, nil

		case "up":
			if m.dirCursor > 0 {
				m.dirCursor--
				return m, nil
			}
		case "down":
			if len(m.directories) > 0 && m.dirCursor < len(m.directories)-1 {
				m.dirCursor++
				return m, nil
			}

		case "b":
			m.state = stateBrowse
			m.browsePath = "/"
			m.browseCursor = 0
			m.loadBrowseDirs("/")
			return m, nil

		case "ctrl+c":
			return m, tea.Quit
		}
	}

	m.dirInput, cmd = m.dirInput.Update(msg)
	return m, cmd
}

// loadBrowseDirs 加载指定路径的子目录
func (m *MainModel) loadBrowseDirs(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		m.browseDirs = nil
		return
	}

	var dirs []dirEntry
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		isSymlink := entry.Type()&os.ModeSymlink != 0
		if entry.IsDir() {
			dirs = append(dirs, dirEntry{Name: name, IsSymlink: isSymlink})
		} else if isSymlink {
			target, err := os.Stat(filepath.Join(path, name))
			if err == nil && target.IsDir() {
				dirs = append(dirs, dirEntry{Name: name, IsSymlink: true})
			}
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	m.browseDirs = dirs
	if m.browseCursor >= len(dirs) {
		m.browseCursor = len(dirs) - 1
	}
	if m.browseCursor < 0 {
		m.browseCursor = 0
	}
}

func (m MainModel) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.browseCursor > 0 {
				m.browseCursor--
			}
			return m, nil

		case "down", "j":
			if m.browseCursor < len(m.browseDirs)-1 {
				m.browseCursor++
			}
			return m, nil

		case "enter":
			if len(m.browseDirs) == 0 {
				return m, nil
			}
			selected := m.browseDirs[m.browseCursor]
			newPath := filepath.Join(m.browsePath, selected.Name)
			m.browsePath = newPath
			m.browseCursor = 0
			m.loadBrowseDirs(newPath)
			return m, nil

		case "backspace", "left", "h":
			if m.browsePath != "/" {
				parent := filepath.Dir(m.browsePath)
				m.browsePath = parent
				m.browseCursor = 0
				m.loadBrowseDirs(parent)
			}
			return m, nil

		case " ":
			// 切换当前目录的选中状态
			path := m.browsePath
			for i, d := range m.directories {
				if d == path {
					m.directories = append(m.directories[:i], m.directories[i+1:]...)
					return m, nil
				}
			}
			m.directories = append(m.directories, path)
			return m, nil

		case "esc", "b":
			m.state = stateInput
			return m, nil

		case "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m MainModel) updateScanning(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m MainModel) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	// 无重复文件时，任意键退出
	if len(m.groups) == 0 {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		handled := true
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.syncViewport()
			}
			m.buildResultsView()
		case "down", "j":
			if m.cursor < m.totalFiles-1 {
				m.cursor++
				m.syncViewport()
			}
			m.buildResultsView()
		case " ":
			path := m.filePathAtIndex(m.cursor)
			if m.selected[path] {
				delete(m.selected, path)
			} else {
				m.selected[path] = true
			}
			m.buildResultsView()
		case "a":
			if len(m.selected) == m.totalFiles {
				m.selected = make(map[string]bool)
			} else {
				for _, g := range m.groups {
					for _, f := range g.Files {
						m.selected[f.Path] = true
					}
				}
			}
			m.buildResultsView()
		case "s":
			m.selected = make(map[string]bool)
			for _, g := range m.groups {
				for i := 1; i < len(g.Files); i++ {
					m.selected[g.Files[i].Path] = true
				}
			}
			m.buildResultsView()
		case "enter":
			if len(m.selected) > 0 {
				m.state = stateConfirm
			}
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			handled = false
		}
		if handled {
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m MainModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			paths := make([]string, 0, len(m.selected))
			for p := range m.selected {
				paths = append(paths, p)
			}
			return m, m.startDelete(paths)
		case "n", "N", "esc":
			m.state = stateResults
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m MainModel) updateDone(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, tea.Quit
	}
	return m, nil
}

// syncViewport 同步 viewport 滚动位置
func (m *MainModel) syncViewport() {
	if m.totalFiles == 0 {
		return
	}
	actualLine := 0
	fileCount := 0
	for i, g := range m.groups {
		actualLine++
		for range g.Files {
			if fileCount == m.cursor {
				goto adjust
			}
			actualLine++
			fileCount++
		}
		if i < len(m.groups)-1 {
			actualLine++
		}
	}
adjust:
	vp := &m.viewport
	if actualLine < vp.YOffset {
		vp.SetYOffset(actualLine)
	} else if actualLine >= vp.YOffset+vp.Height {
		vp.SetYOffset(actualLine - vp.Height + 1)
	}
}

// filePathAtIndex 获取指定全局索引的文件路径
func (m MainModel) filePathAtIndex(idx int) string {
	count := 0
	for _, g := range m.groups {
		for _, f := range g.Files {
			if count == idx {
				return f.Path
			}
			count++
		}
	}
	return ""
}

// startScanWithCh 启动扫描命令
func (m MainModel) startScanWithCh(ch chan scanProgressMsg) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			groups, err := scanner.Scan(m.config.Directories, m.config.Algorithm, func(current, total int, path string) {
				ch <- scanProgressMsg{current: current, total: total, path: path}
			})
			close(ch)
			return scanDoneMsg{groups: groups, err: err}
		},
		waitForProgress(ch),
	)
}

// startDelete 启动删除命令
func (m MainModel) startDelete(paths []string) tea.Cmd {
	return func() tea.Msg {
		results := deleter.DeleteMultiple(paths, m.config.DirectDelete)
		return deleteDoneMsg{results: results}
	}
}

// removeDeletedFiles 从 groups 中移除已删除的文件
func (m *MainModel) removeDeletedFiles(deleted map[string]bool) {
	var filtered []model.DuplicateGroup
	for _, g := range m.groups {
		var remaining []model.FileInfo
		for _, f := range g.Files {
			if !deleted[f.Path] {
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
	m.groups = filtered
	m.totalFiles = 0
	for _, g := range m.groups {
		m.totalFiles += len(g.Files)
	}
}

// buildResultsView 构建结果视图内容
func (m *MainModel) buildResultsView() {
	var b strings.Builder

	groupNum := 0
	globalIdx := 0

	for _, g := range m.groups {
		groupNum++
		b.WriteString(groupStyle.Render(fmt.Sprintf("📋 重复组 #%d (哈希: %s)", groupNum, g.Hash[:16]+"...")))
		b.WriteString("\n")

		for _, f := range g.Files {
			cursor := "  "
			if globalIdx == m.cursor {
				cursor = "▸ "
			}

			checkBox := "[ ]"
			if m.selected[f.Path] {
				checkBox = selectedStyle.Render("[✓]")
			}

			sizeStr := formatSize(f.Size)
			fileLine := fmt.Sprintf("%s%s %s  %s",
				cursor,
				checkBox,
				fileStyle.Render(f.Path),
				highlightStyle.Render(sizeStr),
			)
			b.WriteString(fileLine)
			b.WriteString("\n")
			globalIdx++
		}
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())
}

// renderProgressBar 渲染文字进度条
func renderProgressBar(current, total, width int) string {
	if total <= 0 {
		return ""
	}
	pct := float64(current) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return progressBarStyle.Render(fmt.Sprintf("[%s] %.0f%% (%d/%d)", bar, pct*100, current, total))
}

func (m MainModel) View() string {
	switch m.state {
	case stateInput:
		return m.viewInput()
	case stateBrowse:
		return m.viewBrowse()
	case stateScanning:
		return m.viewScanning()
	case stateResults:
		return m.viewResults()
	case stateConfirm:
		return m.viewConfirm()
	case stateDone:
		return m.viewDone()
	}
	return ""
}

func (m MainModel) viewInput() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 文件去重工具"))
	b.WriteString("\n\n")

	// 已选目录
	if len(m.directories) > 0 {
		b.WriteString(dirSelectedStyle.Render("已选目录:"))
		b.WriteString("\n")
		for i, d := range m.directories {
			prefix := "  "
			if i == m.dirCursor {
				prefix = highlightStyle.Render("▸ ")
			}
			b.WriteString(dirSelectedStyle.Render(prefix + "✓ " + d))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("目录路径（多个路径用英文逗号分隔）:\n")
	b.WriteString(m.dirInput.View())
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("哈希算法: %s (Tab 切换)", highlightStyle.Render(m.algoChoices[m.algoIndex])))
	b.WriteString("\n")

	deleteMode := "删除到回收站"
	if m.directDelete {
		deleteMode = selectedStyle.Render("直接删除")
	}
	b.WriteString(fmt.Sprintf("删除模式: %s (D 切换)", deleteMode))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("❌ %v", m.err)))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("Enter: 开始扫描 | Tab: 切换算法 | D: 切换删除模式 | B: 目录浏览 | ↑/↓: 选择目录 | Delete/x: 移除 | Ctrl+C: 退出"))
	return b.String()
}

func (m MainModel) viewBrowse() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📂 目录浏览"))
	b.WriteString("\n\n")

	// 当前路径
	b.WriteString(infoStyle.Render(fmt.Sprintf("当前: %s", m.browsePath)))
	b.WriteString("\n\n")

	// 已选目录
	if len(m.directories) > 0 {
		b.WriteString(dirSelectedStyle.Render("已选目录:"))
		b.WriteString("\n")
		for _, d := range m.directories {
			b.WriteString(dirSelectedStyle.Render("  ✓ " + d))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// 子目录列表
	if len(m.browseDirs) == 0 {
		b.WriteString(fileStyle.Render("  (无子目录)"))
		b.WriteString("\n")
	} else {
		for i, d := range m.browseDirs {
			cursor := "  "
			if i == m.browseCursor {
				cursor = "▸ "
			}
			icon := "📁"
			if d.IsSymlink {
				icon = "🔗"
			}
			line := fmt.Sprintf("%s%s %s", cursor, icon, d.Name)
			if i == m.browseCursor {
				line = highlightStyle.Render(line)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: 导航 | Enter: 进入 | Space: 添加/移除当前目录 | Backspace: 上级 | Esc/B: 返回输入"))
	return b.String()
}

func (m MainModel) viewScanning() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔍 正在扫描..."))
	b.WriteString("\n\n")

	dirs := strings.Join(m.config.Directories, ", ")
	b.WriteString(fmt.Sprintf("扫描目录: %s\n", dirs))
	b.WriteString(fmt.Sprintf("哈希算法: %s\n\n", m.config.Algorithm))

	if m.progressTotal > 0 {
		barWidth := 40
		if m.width > 0 && m.width-10 < barWidth {
			barWidth = m.width - 10
		}
		if barWidth < 10 {
			barWidth = 10
		}
		b.WriteString(renderProgressBar(m.progressCurrent, m.progressTotal, barWidth))
		b.WriteString("\n")

		if m.progressPath != "" {
			truncated := m.progressPath
			maxLen := 60
			if m.width > 0 {
				maxLen = m.width - 10
			}
			if len(truncated) > maxLen {
				truncated = "..." + truncated[len(truncated)-maxLen+3:]
			}
			b.WriteString(fileStyle.Render(fmt.Sprintf("  当前: %s", truncated)))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(infoStyle.Render("正在收集文件列表..."))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Ctrl+C: 取消"))
	return b.String()
}

func (m MainModel) viewResults() string {
	if len(m.groups) == 0 {
		return titleStyle.Render("✅ 未发现重复文件") + "\n\n按任意键退出"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("📋 发现 %d 组重复文件", len(m.groups))))
	b.WriteString("\n")

	selectedCount := len(m.selected)
	if selectedCount > 0 {
		b.WriteString(infoStyle.Render(fmt.Sprintf("已选择 %d 个文件待删除", selectedCount)))
	}
	b.WriteString("\n")

	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	b.WriteString(helpStyle.Render("↑/k: 上移 | ↓/j: 下移 | Space: 选择 | a: 全选 | s: 保留每组第一个 | Enter: 确认删除 | q: 退出"))
	return b.String()
}

func (m MainModel) viewConfirm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("⚠️  确认删除"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("即将删除 %d 个文件:\n\n", len(m.selected)))

	for path := range m.selected {
		b.WriteString(selectedStyle.Render("  ✗ " + path))
		b.WriteString("\n")
	}

	mode := "移到回收站"
	if m.config.DirectDelete {
		mode = selectedStyle.Render("直接删除（不可恢复）")
	}
	b.WriteString(fmt.Sprintf("\n删除方式: %s\n", mode))

	b.WriteString(helpStyle.Render("\nY: 确认删除 | N/Esc: 取消"))
	return b.String()
}

func (m MainModel) viewDone() string {
	var b strings.Builder

	successCount := 0
	failCount := 0
	for _, r := range m.deleteResults {
		if r.Error == nil {
			successCount++
		} else {
			failCount++
		}
	}

	if failCount == 0 {
		b.WriteString(titleStyle.Render(fmt.Sprintf("✅ 成功删除 %d 个文件", successCount)))
	} else {
		b.WriteString(titleStyle.Render(fmt.Sprintf("⚠️  成功 %d 个，失败 %d 个", successCount, failCount)))
		b.WriteString("\n\n失败详情:")
		for _, r := range m.deleteResults {
			if r.Error != nil {
				b.WriteString(fmt.Sprintf("\n  %s: %s", r.Path, errorStyle.Render(r.Error.Error())))
			}
		}
	}

	b.WriteString(helpStyle.Render("\n\n按任意键退出"))
	return b.String()
}

// formatSize 格式化文件大小为人类可读字符串
func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(KB))
	default:
		return strconv.FormatInt(size, 10) + " B"
	}
}

// Run 启动 TUI 程序
func Run(config model.Config) error {
	m := NewMainModel(config)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
