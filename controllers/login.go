// Package controllers — 登录控制器。
//
// 契约来源：原版 4.9.3 黑盒实测（scripts/probe-original.sh →
// testdata/original/login.json）。
//
//	成功：{"code":0,"message":"ok","type":"success","result":{
//	        "desc":"manager","realName":"admin",
//	        "roles":[{"roleName":"Super Admin","value":"super"}],
//	        "token":"<JWT>","userId":"1","username":"admin"}}
//	失败：{"code":-1,"message":"Incorrect account or password！","result":null,"type":"error"}
//	      （空用户名/密码也走这一条，HTTP 仍是 200）
//
// 反编译地址：Login 0x18ed000 / Logout 0x18ed560 / GetUserInfo 0x18ed5c0。
package controllers

import (
	"vshell/utils"
)

// LoginController 处理 Web 面板登录。
type LoginController struct {
	ApiBaseController
}

// Login 登录（POST /api/login）。
func (c *LoginController) Login() {
	username := c.JsonGetStr("username")
	password := c.JsonGetStr("password")

	// 原版对空用户名/密码也返回同一个错误，不做单独提示。
	token, ok := engineVerifyLogin(username, password)
	if !ok {
		c.Data["json"] = map[string]interface{}{
			"code":    -1,
			"message": "Incorrect account or password！",
			"result":  nil,
			"type":    "error",
		}
		c.ServeJSON()
		return
	}

	engineSetToken(token)
	c.Data["json"] = map[string]interface{}{
		"code":    0,
		"message": "ok",
		"type":    "success",
		"result": map[string]interface{}{
			"desc":     "manager",
			"realName": username,
			"roles": []map[string]interface{}{
				{"roleName": "Super Admin", "value": "super"},
			},
			"token":    token,
			"userId":   "1",
			"username": username,
		},
	}
	c.ServeJSON()
}

// Logout 注销（POST /logout）。原版仅清理会话并返回成功。
func (c *LoginController) Logout() {
	token := c.Ctx.Input.Header("Token")
	if token == "" {
		token = c.GetString("token")
	}
	engineDelToken(token)
	c.JsonOkVerb("logout")
}

// GetUserInfo 返回当前登录用户信息（GET /getUserInfo；router 里另有直接 handler）。
func (c *LoginController) GetUserInfo() {
	c.JsonOkResult(utils.UserInfo{
		Avatar:   "",
		UserID:   "1",
		Username: "admin",
		RealName: "admin",
		Desc:     "manager",
		HomePath: "/dashboard/index",
		Roles:    []utils.RoleInfoResp{{RoleName: "Super Admin", Value: "super"}},
	})
}
