# 面板 API 契约（API）

> 基线：原版 SPA（`static/assets/*.js`）实际发出的请求 + 后端现状。
> 契约以本表为准：**前端按此开发，后端按此实现**。
> 标记：✅ 与前端一致 · ⚠️ 有偏差（见"修复"列） · ❌ 未实现

---

## 0. 约定

- Base：`/api`
- 鉴权：请求头 `Token: <JWT>`（也接受 `Authorization: Bearer`、`X-Token`、`?token=`）
- 请求体：`application/json`（beego 读取 `RequestBody`，参数大小写敏感）
- 响应包：

```jsonc
{"code":0,  "message":"ok", "type":"success", "result":{...}}   // 成功带数据
{"code":0,  "message":"ok", "type":"ok"}                        // 成功无数据
{"code":-1, "message":"...", "result":null, "type":"error"}      // 业务错误
{"code":-1, "result":null}                                       // 鉴权失败（HTTP 401）
```

- 未授权：`Token` 缺失或无效 → HTTP 401（`ApiBaseController.Prepare`，`controllers/base.go:64-95`）

---

## 1. 登录 / 用户

| 方法 | 路径 | 参数 | 响应 | 状态 |
|---|---|---|---|---|
| POST | `/login` | `username`,`password` | `{"token":"<jwt>","userInfo":{...}}` | ✅ 前端直接解构 `{token}` |
| GET/POST | `/logout` | — | `{"code":0,"message":"ok","type":"success"}` | ⚠️ 无服务端状态，等同空操作 |
| GET | `/getUserInfo` | — | `{"code":0,...,"result":{"userId","username","realName","desc","homePath","roles":[{"roleName","value"}],"avatar","token":""}}` | ✅ |
| GET | `/getMenuList` | — | `{"code":0,"result":[{"name","path","icon","component","children":[]}]}` | ⚠️ 只判 token 非空 |
| GET | `/getPermCode` | — | `{"code":0,"result":["*"]}` | ⚠️ 同上，RBAC 未落地 |

JWT claims：`{token_type, user_id, role_id, username, exp, iat}`，有效期 24h，HS256。

---

## 2. 客户端

| 方法 | 路径 | 参数 | 说明 |
|---|---|---|---|
| POST | `/client/list` | `page`,`pageSize`,`status`(0 全部/1 在线/2 离线),`field`,`order`,`search`,`sort` | 分页 |
| POST | `/client/add` | `ip`（支持 `ip:port`） | 手动添加客户端记录 |
| POST | `/client/editremark` | `id`,`remark` | ⚠️ 只改内存，不落库 |
| POST | `/client/delfile` | `id`,`paths`（`\|<-` 分隔） | `rm -rf` |
| POST | `/client/delprocess` | `id`,`pid`,`name` | `kill -9` / `pkill -9` |
| POST | `/client/dellist` | `id`（数组或逗号串） | ⚠️ 数组形式当前解析不到 |

**`/client/list` 响应字段（前端 SPA 实际读取，大小写敏感）**：

| 字段 | 类型 | 当前 | 说明 |
|---|---|---|---|
| `Id` | int | ❌ 给的是 `id` | 行主键 |
| `VerifyKey` | string | ❌ 未返回 | 客户端密钥 |
| `Tp` | string | ❌ 未返回 | 通道类型 |
| `Addr` | string | ❌ 给的是 `ip` | 外网地址 |
| `LocalIP` | string | ❌ 未返回 | 内网 IP |
| `UserName` / `HostName` | string | ❌ 未返回 | 账号 / 主机名 |
| `OsName` / `ProcessName` | string | ❌ 给的是 `os` | 系统 / 进程名 |
| `Location` | string | ❌ 未返回 | 归属地 |
| `IsConnect` | bool | ❌ 未返回 | 是否连接 |
| `PingCheckTime` | int | ❌ 未返回 | 心跳时间 |
| `Remark` | string | ❌ 给的是 `remark` | 备注 |
| `Port` / `DnsPort` | int | ✅ | 端口信息 |
| `version` / `isAdmin` / `arch` / `lastTime` | — | ✅ | 原版 SPA 未使用，可保留 |

外层：`{"clientCount":N,"clientOnlineCount":M,"total":T,"items":[...]}`。
**状态**：R1 已同时输出大写契约键与小写旧键（旧键保留一版以兼容其它调用方）。

---

## 3. 监听器

| 方法 | 路径 | 参数 |
|---|---|---|
| POST | `/listener/list` | `page`,`pageSize`,`status`,`field`,`order`,`search`,`sort` |
| POST | `/listener/add` | `Mode`,`Vkey`,`OssUrl`,`Remark`,`vip`（**需支持 `Host`/`Port`**，见 B0.10） |
| POST | `/listener/edit` | `Id`,`Mode`,`Vkey`,`OssUrl`,`Remark`,`vip` |
| POST | `/listener/editremark` | `id`,`remark` |
| POST | `/listener/del` | `id` |
| POST | `/listener/dellist` | `id[]` |
| POST | `/listener/start` / `stop` | `id` |

`Mode ∈ {http, https, dns, kcp, websocket, cdn_websocket, reverse}`。
列表项：`Id/Mode/Status/OssUrl/Remark/Vkey/Port/Host/Proxy/Salt/vip`。

> ⚠️ 当前 `Add` 把 `Host` 丢掉了（`controllers/engine.go:201`），所以监听地址恒空 → 绑 `0.0.0.0:443`。

---

## 4. 隧道

| 方法 | 路径 | 参数 |
|---|---|---|
| POST | `/tunnel/list` / `add` / `edit` / `editremark` / `del` / `dellist` / `start` / `stop` | `Port`,`Mode`,`Target`,`Remark`,`Id`,`id[]` |

列表项：`Id/Port/Target/Mode/Remark/RunStatus`。⚠️ 无实际转发（B4.1）。

---

## 5. 文件管理

| 方法 | 路径 | 参数 | 当前 |
|---|---|---|---|
| POST | `/file/getdisk` | `id` | ✅ 下发 |
| POST | `/file/ls` | `id`,`path`,`field`,`order` | ⚠️ 回包解析契约不符（`file.go:379-392` 要求 4 个 tab 字段） |
| POST | `/file/cat` | `id`,`path` | ✅ |
| POST | `/file/edit` | `id`,`path`,`content` | ✅ |
| POST | `/file/mv` | `id`,`src`,`dst` | ✅ |
| POST | `/file/rm` / `rmlist` | `id`,`path` / `paths[]` | ✅ |
| POST | `/file/touch` | `id`,`path` | ✅ |
| POST | `/file/mkdir` | `id`,`path` | ✅ |
| POST | `/file/modifytime` | `id`,`path`,`time` | ✅ |
| POST | `/file/wget` | `id`,`url`,`path` | ✅ |
| POST | `/file/upload` | multipart：`file` + `id` + `path` | ❌ **不读文件**（B5.2） |
| POST | `/file/downloadtoserver` | `id`,`path`,`target` | ⚠️ 无进度 |
| GET/POST | `/file/downloadtobrowser` | `id`,`target` | ❌ GET 未注册；无附件输出（B0.7/B5.3） |
| POST | `/file/getdownloadper` | `id` | ⚠️ 恒 0 |
| POST | `/file/downloadtooss` | `id`,`path`,`target` | ❌ 无 OSS 上传 |

---

## 6. 载荷下载

前端用**浏览器导航**（`a.href` + `&token=`）→ **必须是 GET**。

| 方法 | 路径 | 参数 |
|---|---|---|
| GET | `/download/stage` | `id`,`arch` |
| GET | `/download/stageless` | `id`,`arch`,`upx`,`proxy` |
| GET | `/download/listen` | `arch`,`tp`,`host`,`port`,`vkey`,`salt`,`upx` |
| GET | `/download/shellcode` | `id`,`arch` |
| GET | `/download/dll` | `id`,`arch`,`upx`,`proxy`；`arch=loader` 返回 loader 源码 |
| GET | `/download/listendll` | 同 listen + `arch=loader` |

响应：裸二进制 + `Content-Disposition`/`Content-Type`/`Expires: 0`/`Pragma: public`。

> ⚠️ 当前只注册 POST（`router.go:85-90`）→ 前端全部 404。修 B0.7。
> ⚠️ 产物当前不可执行（见 [STATUS.md](STATUS.md) §2.2）。

---

## 7. 安装 / 插件 / 设置 / 截图

| 方法 | 路径 | 参数 | 当前 |
|---|---|---|---|
| POST | `/install/install` / `remove` | `id`,`name` | ⚠️ 下发 `install <name>`，agent 无处理器 |
| GET | `/runner/list` | — | ✅ 读 `./plugins` |
| POST | `/runner/upload` | multipart `file` | ✅ 写 `./plugins` |
| POST | `/runner/runplugin` | `id`,`pluginName`,`procArg` | ❌ 恒 `not aligned` |
| GET | `/setting/get` | — | ⚠️ 只返回 `wx_key` |
| POST | `/setting/edit` | `wx_key` 等 | ⚠️ 内存 map，不落盘 |
| GET | `/screenshot/get` | `id`,`quality` | ❌ agent 不认该命令 |
| GET | `/dashboard/info` | — | ❌ 40+ 键硬编码 |

---

## 8. WebSocket

### 8.1 终端 `GET /api/terminal/ws?id=<clientID>&token=<jwt>`

| 方向 | 帧类型 | 内容 |
|---|---|---|
| 服务端→浏览器 | 文本 | 原始 shell 字节（xterm.js 直接 `write`） |
| 浏览器→服务端 | 文本 | 原始按键字节，或控制 JSON：`{"type":"1","cols":N,"rows":M}` resize、`{"type":"10"}` 关闭 |

服务端把按键 base64 后封装成 `terminal_input:<b64>` 任务下发给 agent。

### 8.2 屏幕 `GET /api/screen/ws?id=<clientID>&quality=<big|normal|small>&token=<jwt>`

| 方向 | 帧类型 | 内容 |
|---|---|---|
| 服务端→浏览器 | **二进制** | zlib 压缩的 PNG（前端 `pako.inflate` 后塞 `<img>`） |
| 浏览器→服务端 | 文本 JSON | 见下表 |

```jsonc
{"type":"1","absX":x,"absY":y,"canvasWidth":w,"canvasHeight":h}  // 鼠标移动
{"type":"2"} {"type":"3","keyCode":k} {"type":"4"}               // 按下/按键/右键
{"type":"5"} {"type":"6"}                                        // 拖拽开始/结束
{"type":"7","direction":"up|down","amount":n}                    // 滚轮
{"type":"8","key":"a","modifiers":[...]}                         // 组合键
{"type":"10"}                                                    // 关闭
```

---

## 9. 与前端 SPA 的差异总表

| # | 差异 | 影响 | 状态 |
|---|---|---|---|
| 1 | 7 个下载端点只有 POST | 下载全 404 | ✅ R1 修复（双注册，e2e 断言 200） |
| 2 | `client/list` 字段名大小写 | 客户端表格空白 | ✅ R1 修复（大写契约字段 + 旧键保留） |
| 3 | `dellist` 不读 JSON 数组 | 批量删除失效 | ✅ R1 修复（`jsonBodyRaw`） |
| 4 | `listener/add` 丢 Host | 监听器只能绑 443 | ✅ R1 修复（`ListenAddr/Host/Port/Salt/...`） |
| 5 | dashboard 假数据 | 首页全假 | ⬜ B5.1 |
| 6 | 上传/插件/截图/下载进度未实现 | 对应页面不可用 | ⬜ B5.2–B5.4 |

---

## 10. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-11-20 | 建立本文：以原版 SPA 实际请求为基线整理全部端点与字段契约 |
