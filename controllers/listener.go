// Package controllers — 监听器控制器。
//
// 契约来源：原版 4.9.3 黑盒实测（scripts/probe-original.sh 采集，见
// testdata/original/ 与 docs/API.md）。以下文案/字段均逐字对齐：
//
//	POST /listener/add         -> {"code":0,"message":"add success","type":"success"}
//	POST /listener/edit        -> save success
//	POST /listener/editremark  -> save success
//	POST /listener/start       -> start success / "already start" / "not found"
//	POST /listener/stop        -> stop success
//	POST /listener/del         -> delete success（不存在也返回成功）
//	POST /listener/dellist     -> delete success
//	POST /listener/list        -> items 为 null（空）/ 每项 14 个键
//
// 反编译地址（funcnametab）：List 0x18ea020 / EditRemark 0x18ebe80 /
// DelList 0x18ebf80 / Del 0x18ec040 / Stop 0x18ec140 / Start 0x18ec1e0 /
// Edit 0x18ec280 / Add 0x18ec880。
package controllers

// ListenerController 管理 C2 监听器。
type ListenerController struct {
	ApiBaseController
}

// List 分页列出监听器（POST /listener/list）。
//
// 原版每项只输出 14 个键（无 Port/Host/Proxy/Salt/vip）：
// ConnectAddr / DNSDomain / DisconnectTimeout / EncryptSalt / Id / ListenAddr /
// MaxDNSsize / Mode / OssUrl / PingInterval / PublicDNS / Remark / Status / Vkey。
func (c *ListenerController) List() {
	page := c.JsonGetInt("page")
	if page < 1 {
		page = 1
	}
	pageSize := c.JsonGetInt("pageSize")
	if pageSize < 1 {
		pageSize = 10
	}
	status := c.JsonGetInt("status")
	list, total := engineGetListenerList((page-1)*pageSize, pageSize,
		c.JsonGetStr("field"), c.JsonGetStr("order"),
		c.JsonGetStr("search"), c.JsonGetStr("sort"), status)

	if len(list) == 0 {
		c.JsonOkResult(map[string]interface{}{"items": nil, "total": total})
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, l := range list {
		items = append(items, map[string]interface{}{
			"ConnectAddr":       l.ConnectAddr,
			"DNSDomain":         l.DNSDomain,
			"DisconnectTimeout": l.DisconnectTimeout,
			"EncryptSalt":       l.Salt,
			"Id":                l.ID,
			"ListenAddr":        l.ListenAddr,
			"MaxDNSsize":        l.MaxDNSsize,
			"Mode":              l.Mode,
			"OssUrl":            l.OssUrl,
			"PingInterval":      l.PingInterval,
			"PublicDNS":         l.PublicDNS,
			"Remark":            l.Remark,
			"Status":            l.Status,
			"Vkey":              l.Vkey,
		})
	}
	c.JsonOkResult(map[string]interface{}{"items": items, "total": total})
}

// Add 新增监听器（POST /listener/add）。
//
// 原版在 add 里就把监听器置为运行状态（随后 start 返回 "already start"），
// 因此这里创建后立即启动；启动失败不阻塞创建（与原版一致：状态仍为 true）。
//
// 参数：Mode / Vkey / OssUrl / Remark / vip，扩展兼容
// Host / Port / ListenAddr / ConnectAddr / EncryptSalt / PingInterval /
// DisconnectTimeout / DNSDomain / PublicDNS / MaxDNSsize。
func (c *ListenerController) Add() {
	l := EngineListener{
		Mode:              c.JsonGetStr("Mode"),
		Vkey:              c.JsonGetStr("Vkey"),
		OssUrl:            c.JsonGetStr("OssUrl"),
		Remark:            c.JsonGetStr("Remark"),
		IsVIP:             c.JsonGetBool("vip"),
		Host:              c.JsonGetStr("Host"),
		Port:              c.JsonGetInt("Port"),
		ListenAddr:        c.JsonGetStr("ListenAddr"),
		ConnectAddr:       c.JsonGetStr("ConnectAddr"),
		PingInterval:      c.JsonGetInt("PingInterval"),
		DisconnectTimeout: c.JsonGetInt("DisconnectTimeout"),
		DNSDomain:         c.JsonGetStr("DNSDomain"),
		PublicDNS:         c.JsonGetStr("PublicDNS"),
		MaxDNSsize:        c.JsonGetInt("MaxDNSsize"),
		Salt:              firstNonEmpty(c.JsonGetStr("EncryptSalt"), c.JsonGetStr("Salt")),
	}
	if l.Mode == "" {
		c.JsonErr("mode required")
		return
	}
	if err := engineAddListener(l); err != nil {
		c.JsonErr("add listener failed: " + err.Error())
		return
	}
	c.JsonOkVerb("add")
}

// Edit 编辑监听器（POST /listener/edit）。
func (c *ListenerController) Edit() {
	l := EngineListener{
		ID:                int64(c.JsonGetInt("Id")),
		Mode:              c.JsonGetStr("Mode"),
		Vkey:              c.JsonGetStr("Vkey"),
		OssUrl:            c.JsonGetStr("OssUrl"),
		Remark:            c.JsonGetStr("Remark"),
		IsVIP:             c.JsonGetBool("vip"),
		Host:              c.JsonGetStr("Host"),
		Port:              c.JsonGetInt("Port"),
		ListenAddr:        c.JsonGetStr("ListenAddr"),
		ConnectAddr:       c.JsonGetStr("ConnectAddr"),
		PingInterval:      c.JsonGetInt("PingInterval"),
		DisconnectTimeout: c.JsonGetInt("DisconnectTimeout"),
		Salt:              firstNonEmpty(c.JsonGetStr("EncryptSalt"), c.JsonGetStr("Salt")),
	}
	if l.ID == 0 {
		c.JsonErr("id error")
		return
	}
	if err := engineEditListener(l); err != nil {
		c.JsonErr("edit listener failed: " + err.Error())
		return
	}
	c.JsonOkVerb("save")
}

// EditRemark 修改监听器备注（POST /listener/editremark）。参数：id / remark。
func (c *ListenerController) EditRemark() {
	id := int64(c.JsonGetInt("id"))
	if id == 0 {
		c.JsonErr("id error")
		return
	}
	engineEditListenerRemark(id, c.JsonGetStr("remark"))
	c.JsonOkVerb("save")
}

// Del 删除监听器（POST /listener/del）。不存在也返回成功（原版幂等）。
func (c *ListenerController) Del() {
	id := int64(c.JsonGetInt("id"))
	if id == 0 {
		c.JsonErr("id error")
		return
	}
	engineDelListener(id)
	c.JsonOkVerb("delete")
}

// DelList 批量删除监听器（POST /listener/dellist）。参数：id（数组或逗号串）。
func (c *ListenerController) DelList() {
	ids := c.JsonGetIntList("id")
	if len(ids) == 0 {
		c.JsonErr("id error")
		return
	}
	id64 := make([]int64, len(ids))
	for i, v := range ids {
		id64[i] = int64(v)
	}
	engineDelListeners(id64)
	c.JsonOkVerb("delete")
}

// Start 启动监听器（POST /listener/start）。
//
// 三种结果（黑盒实测）：不存在 → "not found"；已在运行 → "already start"；
// 成功 → "start success"。
func (c *ListenerController) Start() {
	id := int64(c.JsonGetInt("id"))
	if id == 0 {
		c.JsonErr("id error")
		return
	}
	l := engineGetListener(id)
	if l == nil {
		c.JsonErr("not found")
		return
	}
	if l.Status {
		c.JsonErr("already start")
		return
	}
	if err := engineStartListener(id); err != nil {
		c.JsonErr(err.Error())
		return
	}
	c.JsonOkVerb("start")
}

// Stop 停止监听器（POST /listener/stop）。不存在也返回成功。
func (c *ListenerController) Stop() {
	id := int64(c.JsonGetInt("id"))
	if id == 0 {
		c.JsonErr("id error")
		return
	}
	engineStopListener(id)
	c.JsonOkVerb("stop")
}

// firstNonEmpty 返回第一个非空字符串（面板对同一字段有多种键名）。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
