# duplicate-cleaner

文件去重工具。

```mermaid
graph LR
    a[-l 列出重复文件] --> b[手动维护待删除清单] --> c[-c 删除指定文件]
```

```sh
# 列出重复文件
duplicate-cleaner -l [-f [md5 | sha1 | sha256 | sha512]] [-n num] [-o file] dir1 [dir2 ...]

# 删除指定文件
duplicate-cleaner -c file1 [file2 ...]
```

## TODO

- [ ] 删除到`回收站`
