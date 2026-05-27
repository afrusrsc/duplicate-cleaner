/*
Copyright (c) 2025 Jesse Jin Authors. All rights reserved.

Use of this source code is governed by a MIT-style
license that can be found in the LICENSE file.

版权由作者 Jesse Jin <afrusrsc@126.com> 所有。
此源码的使用受 MIT 开源协议约束，详见 LICENSE 文件。
*/

// Package model 定义了文件去重工具的核心数据模型。
package model

// FileInfo 记录单个文件的路径、大小和哈希值。
type FileInfo struct {
	Path string // 文件完整路径
	Size int64  // 文件大小（字节）
	Hash string // 文件哈希值（十六进制）
}

// DuplicateGroup 表示一组哈希值相同的重复文件。
type DuplicateGroup struct {
	Hash  string     // 该组共有的哈希值
	Files []FileInfo // 属于该组的所有文件
}

// Config 是应用程序的配置参数。
type Config struct {
	Directories  []string // 要扫描的目录路径列表
	Algorithm    string   // 哈希算法: md5, sha1, sha256, sha512
	DirectDelete bool     // true=直接删除, false=删除到回收站
	Port         int      // Web 服务端口
}
