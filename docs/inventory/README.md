# 原版二进制清单（inventory）

本目录存放从**本机原版 4.9.3（Linux 构建）** `/home/kali/go/bin/vshell` 静态恢复的
符号与地址清单，供契约对齐与定点逆向使用。

| 文件 | 内容 | 生成方式 |
|---|---|---|
| `vshell-4.9.3-linux-syms.txt` | 38645 个函数：`地址 名字`（其中 11699 个有名字） | `scripts/go-symbols.py` |
| `controllers.md` | 13 个业务控制器共 239 个方法的地址与字节数 | 从上一份清单筛出 `J30c5QDQ0w.(*XxxController).Yyy` |

## 为什么能恢复

原版是 garble 混淆过的 Go 构建：

- ELF 符号表被剥离（`go tool nm` 报 `no symbols`）；
- `.gopclntab` 段**仍然存在**（6.3 MB），但头部魔数被改成非标准值（实测 `0x840c4ea6`）；
- 包名被随机化（`J30c5QDQ0w`、`Uq50nq2sva`、`HsxvQGdEn` …）；
- **类型名与方法名保留**（例如 `(*DownloadController).Dll`），因此"按名字找函数"可行；
- 字符串字面量被加密（`-literals`），`.rodata` 里看到的是密文（如 `b'idifinipki...'`），
  静态读不到明文 —— 这是后面要做**动态分析**的原因。

`scripts/go-symbols.py` 不依赖 Go 版本串、也不校验魔数，只按 pcHeader 的字段宽度解析，
因此对 garble 改魔数的二进制同样适用。

## 怎么用

```bash
# 1) 恢复整张符号表（约 2 秒）
scripts/go-symbols.py /home/kali/go/bin/vshell > /tmp/syms.txt

# 2) 找某个控制器的全部方法
grep "DownloadController" /tmp/syms.txt

# 3) 反汇编某个函数并把调用目标标上名字（省略长度则取到下一个函数）
scripts/disas.py /home/kali/go/bin/vshell 0x1822d80 448     # ClientController.DelFile

# 4) 按符号名反汇编
scripts/disas.py /home/kali/go/bin/vshell ClientController.DelFile
```

`scripts/disas.py` 的注释列会显示：
- `call` 目标属于哪个函数（`; J30c5QDQ0w.(*ClientController).Add+0x20`）；
- `lea rXX,[rip+…]` 命中的字符串常量（明文可读时直接显示）。

## 已知限制

| 限制 | 说明 | 绕法 |
|---|---|---|
| 字符串是密文 | garble `-literals`：每个字面量用独立种子加密，运行时才解密 | gdb 断点动态取值；或复用已恢复的 `dst-src` 池解码（E-14） |
| 地址是 Linux 构建 | 与 `.re` 里的 Windows `FUN_01xxxxxx` 不同 | 跨版本只按"类型名/方法名/字符串"对应，不搬地址 |
| 空名字函数 | 26946 个函数名字被抹掉 | 用"包含关系"归属（地址落在哪个命名函数区间内） |

## 内嵌 agent 模板

外层二进制里还内嵌了 40+ 个 agent 镜像（PE amd64/i386 + ELF amd64/arm64，单个 5–12 MB）：

```bash
scripts/extract-templates.py /home/kali/go/bin/vshell               # 列清单
scripts/extract-templates.py /home/kali/go/bin/vshell --out /tmp/tpl # 提取
```

提取出来的 PE/ELF 就是面板"下载载荷"要发的模板（`stageless/<tp>_<arch>` / `dll/<tp>_<arch>`），
可以直接当 M2 的回归素材；模板自身也能用 `scripts/go-symbols.py` 恢复符号。

## 下一步（动态）

1. `gdb -p <pid>`（或起第二份实例）+ `break *0x1822d80`，从面板触发对应接口，
   在 `JsonGetStr`/`JsonGetInt` 返回处读寄存器，直接拿到**解密后的参数名**；
2. 对 `DownloadController.*` 下断点，抓它拼出的模板路径（`stageless/<tp>_<arch>` 已由
   黑盒错误信息确认，动态可再抓配置块字段）；
3. 对 agent 侧的任务分派断点，抓"面板命令 → opcode 记录"的真实编码（M1 的 B1.1/B1.3）。
