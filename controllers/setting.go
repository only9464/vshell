// Package controllers — 设置控制器，根据当前逆向证据重建；行为等价性仍需逐项验证。
//
// 真实方法（funcnametab + 反编译地址）与前端路由：
//
//	Edit  0x18f02e0   POST /setting/edit  保存设置
//
// 反编译参数集：wx_key（企业微信机器人 key，通知配置）。
package controllers

// SettingController 管理面板设置。
type SettingController struct {
	ApiBaseController
}

// Edit 保存设置（POST /setting/edit）。
// 反编译（0x18f02e0）读取 wx_key 等通知配置并持久化。
func (c *SettingController) Edit() {
	// 黑盒：setting/edit 返回 {"code":0,"message":"modified success","type":"success"}。
	if v := c.JsonGetStr("wx_key"); v != "" {
		engineSaveSetting("wx_key", v)
	}
	if v := c.JsonGetStr("dingding_access_token"); v != "" {
		engineSaveSetting("dingding_access_token", v)
	}
	if v := c.JsonGetStr("dingding_key_word"); v != "" {
		engineSaveSetting("dingding_key_word", v)
	}
	c.JsonOkVerb("modified")
}

// Get 返回设置（GET /setting/get；前端内嵌路由 url:"/setting/get"）。
func (c *SettingController) Get() {
	// 黑盒：原版只回这三个键（含空串）。
	c.JsonOkResult(map[string]interface{}{
		"dingding_access_token": engineGetSetting("dingding_access_token"),
		"dingding_key_word":     engineGetSetting("dingding_key_word"),
		"wx_key":                engineGetSetting("wx_key"),
	})
}

// ---- 引擎阶段对齐 ----

var engineSettings = map[string]string{}

func engineSaveSetting(key, value string) {
	engineSettings[key] = value
}

func engineGetSetting(key string) string {
	return engineSettings[key]
}
