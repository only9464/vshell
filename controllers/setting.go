// Package controllers — 设置控制器。
//
// 契约（原版 4.9.3 黑盒）：
//
//	GET  /setting/get  → {"code":0,...,"result":{"dingding_access_token":"","dingding_key_word":"","wx_key":""}}
//	POST /setting/edit → {"code":0,"message":"modified success","type":"success"}
//
// 存续：写回 conf/setting.conf（`utils.UpdateConfig`），不再只是内存 map——
// 之前重启即丢，面板"设置"形同虚设。
package controllers

import (
	"log"

	"vshell/utils"
)

// SettingController 管理面板设置。
type SettingController struct {
	ApiBaseController
}

// Edit 保存设置（POST /setting/edit）。
func (c *SettingController) Edit() {
	updates := map[string]interface{}{}
	for _, k := range []string{"wx_key", "dingding_access_token", "dingding_key_word"} {
		if v := c.JsonGetStr(k); v != "" {
			updates[k] = v
		}
	}
	if len(updates) == 0 {
		// 原版对空提交同样返回成功；这里保持一致，但不写盘。
		c.JsonOkVerb("modified")
		return
	}
	if err := utils.UpdateConfig(updates, settingConfPath()); err != nil {
		log.Printf("[setting] persist failed: %v", err)
		c.JsonErr("save setting failed: " + err.Error())
		return
	}
	c.JsonOkVerb("modified")
}

// Get 返回设置（GET /setting/get；原版只回这三个键）。
func (c *SettingController) Get() {
	cfg := utils.GetFullSettings()
	c.JsonOkResult(map[string]interface{}{
		"dingding_access_token": cfg.DingdingAccessToken,
		"dingding_key_word":     cfg.DingdingKeyWord,
		"wx_key":                cfg.WxKey,
	})
}

// settingConfPath 返回设置文件路径（与原版一致：conf/setting.conf）。
func settingConfPath() string { return "conf/setting.conf" }
