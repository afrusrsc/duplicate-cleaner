/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"duplicate-cleaner/internal/hasher"
	"duplicate-cleaner/internal/model"
	"duplicate-cleaner/internal/tui"
	"duplicate-cleaner/internal/web/handlers"
)

func main() {
	// 命令行参数
	var (
		dirs         multiFlag
		algo         string
		tuiMode      bool
		directDelete bool
		port         int
	)

	flag.Var(&dirs, "d", "要扫描的目录路径（可重复使用，例如 -d /path1 -d /path2）")
	flag.Var(&dirs, "dir", "要扫描的目录路径（可重复使用）")
	flag.StringVar(&algo, "a", "md5", "哈希算法: md5|sha1|sha256|sha512")
	flag.StringVar(&algo, "algorithm", "md5", "哈希算法: md5|sha1|sha256|sha512")
	flag.BoolVar(&tuiMode, "t", false, "使用 TUI 终端界面")
	flag.BoolVar(&tuiMode, "tui", false, "使用 TUI 终端界面")
	flag.BoolVar(&directDelete, "direct-delete", false, "直接删除（不放入回收站）")
	flag.IntVar(&port, "p", 8080, "Web 服务端口")
	flag.IntVar(&port, "port", 8080, "Web 服务端口")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "文件去重工具 - 查找并删除重复文件\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n  duplicate-cleaner [选项]\n\n")
		fmt.Fprintf(os.Stderr, "选项:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  duplicate-cleaner                        # 启动 Web 界面\n")
		fmt.Fprintf(os.Stderr, "  duplicate-cleaner -t                     # 启动 TUI 界面\n")
		fmt.Fprintf(os.Stderr, "  duplicate-cleaner -d /path1 -d /path2    # 指定多个扫描目录\n")
		fmt.Fprintf(os.Stderr, "  duplicate-cleaner -a sha256              # 使用 SHA-256 算法\n")
		fmt.Fprintf(os.Stderr, "  duplicate-cleaner --direct-delete        # 直接删除模式\n")
	}

	flag.Parse()

	// 验证哈希算法
	if !hasher.ValidAlgorithm(algo) {
		fmt.Fprintf(os.Stderr, "错误: 不支持的哈希算法 %q（可选: md5, sha1, sha256, sha512）\n", algo)
		os.Exit(1)
	}

	config := model.Config{
		Directories:  dirs,
		Algorithm:    algo,
		DirectDelete: directDelete,
		Port:         port,
	}

	if tuiMode {
		// TUI 模式
		if err := tui.Run(config); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Web 模式
		addr := fmt.Sprintf(":%d", port)
		router := handlers.NewRouter()

		fmt.Printf("🔍 文件去重工具已启动\n")
		fmt.Printf("   访问 http://localhost%s\n", addr)
		fmt.Printf("   算法: %s | 删除模式: %s\n", algo, deleteModeStr(directDelete))
		if len(dirs) > 0 {
			fmt.Printf("   预设目录: %v\n", dirs)
		}
		fmt.Println()

		// 自动打开浏览器
		url := fmt.Sprintf("http://localhost%s", addr)
		go func() {
			// 等待服务器启动
			for i := 0; i < 10; i++ {
				resp, err := http.Get(url)
				if err == nil {
					resp.Body.Close()
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			openBrowser(url)
		}()

		log.Fatal(http.ListenAndServe(addr, router))
	}
}

// openBrowser 跨平台打开浏览器。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // linux 及其他 unix
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// deleteModeStr 返回删除模式的描述文本。
func deleteModeStr(direct bool) string {
	if direct {
		return "直接删除"
	}
	return "移到回收站"
}

// multiFlag 支持多次使用的 flag（如 -d dir1 -d dir2）。
type multiFlag []string

func (m *multiFlag) String() string {
	return fmt.Sprintf("%v", []string(*m))
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
