# 🔍 文件去重工具 (duplicate-cleaner)

查找并删除重复文件的命令行工具，支持 Web 界面和 TUI 终端界面。跨平台支持 Linux、macOS 和 Windows。

## ✨ 功能特性

- **多算法支持** — MD5、SHA-1、SHA-256、SHA-512，默认 MD5
- **双界面模式** — Web 浏览器界面（默认）和 TUI 终端界面（`-t`）
- **并行扫描** — 并发计算文件哈希，实时显示进度条
- **结果分组** — 按哈希值分组列出重复文件，显示完整路径和文件大小
- **安全删除** — 默认删除到回收站，支持直接删除模式
- **目录浏览** — Web 和 TUI 均支持目录浏览器选择目录（含软链接），也可手动输入路径

## 📦 安装与构建

### 前置要求

- Go 1.24+
- [Templ](https://templ.guide/) CLI（用于编译模板）

### 编译

```bash
# 安装 Templ CLI
go install github.com/a-h/templ/cmd/templ@latest

# 生成模板代码
templ generate ./...

# 编译
# Linux / macOS
go build -o duplicate-cleaner .
# Windows
go build -o duplicate-cleaner.exe .
```

## 🚀 使用方法

### Web 模式（默认）

```bash
# 启动 Web 界面，自动打开浏览器（Linux/macOS/Windows 均支持）
duplicate-cleaner

# 指定端口
duplicate-cleaner -p 3000

# 预设扫描目录
duplicate-cleaner -d /path/to/dir1 -d /path/to/dir2

# 指定算法
duplicate-cleaner -a sha256

# 直接删除模式
duplicate-cleaner --direct-delete
```

启动后自动打开系统浏览器访问 `http://localhost:8080`。

### TUI 模式

```bash
# 启动终端界面
duplicate-cleaner -t
```

TUI 操作：

| 按键 | 功能 |
|------|------|
| `↑/↓` | 在已选目录列表中移动光标 |
| `Delete` / `x` | 移除光标所在的已选目录 |
| `Tab` | 切换哈希算法 |
| `D` | 切换删除模式（回收站/直接删除） |
| `B` | 进入/退出目录浏览模式 |
| `Enter` | 开始扫描 / 确认删除 |
| `Space` | 选择文件 / 添加移除目录 |
| `a` | 全选/取消全选 |
| `s` | 保留每组第一个文件 |
| `q` | 退出 |
| `Ctrl+C` | 强制退出 |

### 命令行参数

```
用法:
  duplicate-cleaner [选项]

选项:
  -d, --dir <path>         要扫描的目录路径（可重复使用）
  -a, --algorithm <algo>   哈希算法: md5|sha1|sha256|sha512（默认 md5）
  -t, --tui                使用 TUI 终端界面
      --direct-delete      直接删除（不放入回收站）
  -p, --port <port>        Web 服务端口（默认 8080）
  -h, --help               显示帮助信息
```

## 🛠️ 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go |
| Web 界面 | net/http + [Templ](https://templ.guide/) + [Tailwind CSS](https://tailwindcss.com/) + [Alpine.js](https://alpinejs.dev/) |
| TUI 界面 | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) |
| 哈希算法 | crypto/md5, crypto/sha1, crypto/sha256, crypto/sha512 |
| 回收站 | [go-trash](https://github.com/natefinch/go-trash) |

## 📁 项目结构

```
duplicate-cleaner/
├── main.go                          # 入口，命令行参数解析
├── internal/
│   ├── model/model.go               # 数据模型定义
│   ├── hasher/hasher.go             # 哈希计算（多算法支持）
│   ├── scanner/scanner.go           # 文件扫描与重复检测
│   ├── deleter/deleter.go           # 文件删除（回收站/直接删除）
│   ├── tui/tui.go                   # TUI 终端界面
│   └── web/
│       ├── handlers/handler.go      # Web HTTP 路由与处理
│       └── views/                   # Templ 模板
│           ├── layout.templ         # 布局模板
│           ├── index.templ          # 首页（目录选择、配置）
│           └── results.templ        # 结果页（重复文件列表）
├── go.mod
├── go.sum
└── LICENSE                          # MIT 许可证
```

## 📄 许可证

[MIT License](LICENSE)

Copyright (c) 2025 Jesse Jin <afrusrsc@126.com>