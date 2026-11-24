# 发版检查清单

一条命令跑完所有门禁：

```bash
./scripts/release-check.sh
```

它依次执行 9 项，逐项打印 ✓/✗ 与耗时，最后给出"能不能发"的结论：

| # | 门禁 | 说明 |
|---|---|---|
| 1 | 服务端构建 | `go build ./...` |
| 2 | 静态检查 | `go vet ./...` |
| 3 | 服务端测试 | `go test ./... -count=1`（264 个测试函数） |
| 4 | 竞态检测 | `go test -race`（R41 抓到过 3 处生产代码竞争，必须常跑） |
| 5 | agent 构建/测试 | `cd agent && go build && go test`（78 个测试函数） |
| 6 | 前端构建 | `cd web2 && pnpm build`（产物落到 `web2/dist`，服务端 `/app/` 优先托管它；`web/` 为上一版对照） |
| 7 | API 面对齐 | `scripts/api-parity.py`：对照原版前端包，**缺失必须为 0** |
| 8 | 端到端 | `scripts/e2e.sh`（四条传输 + 面板契约 + 隧道 + 下载 + 插件 + MCP） |
| 9 | 前端渲染/点击 | `scripts/ui-check.sh`（11 个页面渲染 + 10 条点击级断言） |

跳过某项（例如无 chromium 的机器）：`SKIP_UI=1` / `SKIP_E2E=1`。

---

## 人工确认项（脚本覆盖不到的）

- [ ] **文档与实现一致**：改了接口就更新 [API.md](API.md)，改了协议就更新 [PROTOCOL.md](PROTOCOL.md)，
      改了差距就更新 [GAPS.md](GAPS.md)
- [ ] **CHANGELOG/ROADMAP**：本轮改动写进 [ROADMAP.md](ROADMAP.md) 的变更表
- [ ] **不提交构建产物**：`web2/dist`、`web/dist`、`logs/`、`db/`、临时目录不入库（见 `.gitignore`）
- [ ] **配置示例可跑**：按 [RELEASE.md](RELEASE.md) 从零起一个实例，能登录、能上线 agent
- [ ] **默认凭据安全**：`conf/setting.conf` 里没有真实密码/密钥；`web_jwt_secret` 建议留空
- [ ] **已知限制仍准确**：官方 agent 的 `main` 通道、未实现的通知/OSS 等仍写在 [GAPS.md](GAPS.md)

## 发版步骤

```bash
# 1) 门禁
./scripts/release-check.sh

# 2) 打 tag（示例）
git tag -a v0.x.0 -m "v0.x.0: <一句话说明>"
git push origin main --tags
```

部署见 [RELEASE.md](RELEASE.md)。
