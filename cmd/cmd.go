/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

package cmd

import (
	"bufio"
	"duplicate-cleaner/duplicate"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type Config struct {
	hash    string
	list    bool
	clean   bool
	outFile string
	count   int
	args    []string
}

const splitLine = "--------"

// Execute 运行命令行
func Execute() {
	cfg := parseConfig()
	if err := checkConfig(cfg); err != nil {
		fmt.Println(err)
		return
	}
	if cfg.list {
		if err := list(cfg); err != nil {
			fmt.Println(err)
		}
		return
	}
	if cfg.clean {
		if err := clean(cfg); err != nil {
			fmt.Println(err)
		}
		return
	}
}

// list 列出重复文件
func list(cfg *Config) error {
	l, err := duplicate.List(cfg.args, cfg.hash, cfg.count)
	if err != nil {
		return err
	}
	if err := saveList(cfg.outFile, l); err != nil {
		return err
	}
	return nil
}

// saveList 保存重复清单
func saveList(f string, l duplicate.DupList) error {
	if len(l) == 0 {
		return errors.New("无重复文件")
	}
	file, err := os.OpenFile(f, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := io.MultiWriter(file, os.Stdout)
	for _, v := range l {
		io.WriteString(writer, splitLine+"\n")
		for _, s := range v {
			io.WriteString(writer, fmt.Sprintf("%s\t%dB\t%s\n", s.Path, s.Size, s.Hash))
		}
	}
	return nil
}

// clean
func clean(cfg *Config) error {
	delList, err := readList(cfg.args)
	if err != nil {
		return err
	}
	n, err := duplicate.Clean(delList)
	if err != nil {
		return err
	}
	fmt.Printf("成功清理 %d 个文件", n)
	return nil
}

// readList 读取删除清单
func readList(files []string) ([]string, error) {
	delList := []string{}
	for _, f := range files {
		file, err := os.OpenFile(f, os.O_RDONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("无法打开文件 %s: %v", f, err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if line != splitLine {
				s := strings.Split(line, "\t")
				if len(s) < 1 {
					return nil, fmt.Errorf("文件 %s 格式错误: 每行应包含文件路径", f)
				}
				delList = append(delList, s[0])
			}
		}
	}
	return delList, nil
}

// checkConfig 检查参数
func checkConfig(cfg *Config) error {
	if (cfg.list && cfg.clean) || (!cfg.list && !cfg.clean) {
		return errors.New("-l 和 -c 必须二选一")
	}
	if cfg.count < 1 {
		return errors.New("同时计算数不能小于1")
	}
	if len(cfg.args) == 0 {
		if cfg.list {
			return errors.New("请指定待分析的路径")
		}
		if cfg.clean {
			return errors.New("请指定待清理文件的列表")
		}
	}
	return nil
}

// parseConfig 解析命令行参数
func parseConfig() *Config {
	cfg := Config{}

	flag.BoolVar(&cfg.list, "l", false, "列出重复文件清单，与 -c 必须二选一")
	flag.StringVar(&cfg.hash, "f", "md5", "比较方式: md5 | sha1 | sha256 | sha512")
	flag.StringVar(&cfg.outFile, "o", "list.txt", "将重复清单输出到指定文件")
	flag.IntVar(&cfg.count, "n", 10, "同时计算数量")
	flag.BoolVar(&cfg.clean, "c", false, "清理指定的文件，与 -l 必须二选一")

	flag.Parse()

	cfg.args = flag.Args()

	return &cfg
}
