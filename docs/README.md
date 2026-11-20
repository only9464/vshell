# VShell 复刻项目文档

本目录是本仓库的**唯一事实来源**（single source of truth）。根目录 README 只做简介与导航，
任何"当前实现到什么程度"的判断以本目录为准。

## 文档索引

| 文档 | 内容 | 谁该读 |
|---|---|---|
| [ROADMAP.md](ROADMAP.md) | 里程碑计划、任务清单、验收标准、依赖与风险 | 所有人；每轮开发前先看这里 |
| [STATUS.md](STATUS.md) | 还原度矩阵、已实现/部分/桩的逐项判定（附 `文件:行` 证据） | 想知道"现在能跑什么"的人 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | 模块划分、进程结构、数据流、代码地图 | 新加入的开发 |
| [PROTOCOL.md](PROTOCOL.md) | 线协议：传输帧、KCP 链路、DNS 信道、任务/结果记录 | 改协议、写 agent、做互通测试 |
| [API.md](API.md) | 面板 HTTP/WS API 契约 + 前端字段契约 | 后端与前端开发者 |
| [FRONTEND.md](FRONTEND.md) | Vue3 重开发方案（技术选型、目录、页面、联调） | 前端开发者 |
| [DEV.md](DEV.md) | 构建、运行、调试、测试、端到端联调 | 所有人 |
| [RE-NOTES.md](RE-NOTES.md) | 逆向结论与证据分级（哪些是硬证据、哪些是推断、哪些未恢复） | 做对齐的人 |
| [REFERENCE.md](REFERENCE.md) | 本机原版 4.9.3 实例的取证流程、已确认行为（R-01…R-16）、工具链 | 做对齐与逆向的人 |
| [inventory/](inventory/README.md) | 原版二进制的符号表与控制器方法地址清单（`scripts/go-symbols.py`、`scripts/disas.py`） | 做逆向/定点分析的人 |

## 三条硬规则

1. **不臆造**：任何与原版相关的实现，必须在代码注释里标注证据来源（反编译地址 / 抓包 /
   黑盒响应）。没有证据的自造实现要么删除，要么明确标 `PLACEHOLDER` 并登记到
   [STATUS.md](STATUS.md) 的"占位清单"。
2. **端到端可验证**：每个里程碑都必须有 `scripts/e2e.sh` 或等价测试证明"面板 → 引擎 → agent
   → 回显"整条链路，单元测试全绿不算完成。
3. **文档随代码走**：改了协议/接口/行为，同一次改动内更新本目录对应文档与 [STATUS.md](STATUS.md)。

## 与原版的关系

本仓库是对 VShell v3.0（`v_windows_amd64.exe`，garble 混淆的 Go 二进制）的独立重新实现研究，
与原版作者无关、不含原版许可。目标不是"分发一个可用的 C2"，而是**把逆向结论落成可验证的
代码**并对齐到功能等价。所有"对齐"声明都必须能在 [RE-NOTES.md](RE-NOTES.md) 找到证据出处。
