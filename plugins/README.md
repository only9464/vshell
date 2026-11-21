# 服务器插件目录

面板「插件」页会列出本目录下的文件，点「执行」后由**目标机**下载并运行，输出回显到页面。

## 支持的扩展名

| 扩展名 | 平台 |
|---|---|
| `.elf` | Linux（本仓库主要验证环境） |
| `.so` | Linux 动态库 |
| `.dylib` | macOS |

其他后缀会返回 `support .elf,.so,.dylib executable, extensions`（与原版一致）。

## 内置示例：`sysinfo`

源码在 `plugins/sysinfo/`，构建后拷到本目录即可：

```bash
(cd plugins/sysinfo && go build -ldflags="-s -w" -o ../../plugins/sysinfo.elf .)
```

执行效果（面板里点「执行」）：

```
host=kali os=linux/amd64
user=kali uid=1000
pid=12345 time=2026-11-21T22:13:31+08:00
```

## 注意

- 插件是在**被控端**执行，因此要为对应架构编译（`GOOS/GOARCH`）。
- 目标机需要能执行该文件（`chmod +x` 由 agent 处理）。
