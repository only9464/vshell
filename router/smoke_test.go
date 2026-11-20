package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"vshell/utils"
)

// TestRealAPIEndpoints smoke-tests the real API surface end to end:
// login exempt, protected endpoints 401 without token, and the JSON
// envelope shape of the recovered controllers.
func TestRealAPIEndpoints(t *testing.T) {
	// SPA/static paths are cwd-relative; run from the repo root
	if _, err := os.Stat("static/index.html"); err != nil {
		if _, err := os.Stat("../static/index.html"); err == nil {
			_ = os.Chdir("..")
		}
	}
	mux := InitRouter()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. /api/login is public (no token needed) — returns the login envelope
	resp, err := http.Post(srv.URL+"/api/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"wrong"}`))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("login should be public, got 401")
	}
	t.Logf("login response: %s", body)

	// 1b. success login with the configured password → JWT token
	cfg := utils.GetFullSettings()
	if cfg.WebPassword != "" {
		respL, err := http.Post(srv.URL+"/api/login", "application/json",
			strings.NewReader(`{"username":"`+cfg.WebUsername+`","password":"`+cfg.WebPassword+`"}`))
		if err != nil {
			t.Fatalf("login ok: %v", err)
		}
		defer respL.Body.Close()
		bL, _ := io.ReadAll(respL.Body)
		var loginResp map[string]interface{}
		if err := json.Unmarshal(bL, &loginResp); err != nil {
			t.Fatalf("login ok non-JSON: %s", bL)
		}
		tokL, _ := loginResp["token"].(string)
		if tokL == "" {
			t.Fatalf("login ok missing token: %s", bL)
		}
		t.Logf("login ok token: %.20s...", tokL)

		// authorized call with the login-issued token
		reqA, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/tunnel/list", nil)
		reqA.Header.Set("Token", tokL)
		respA, err := http.DefaultClient.Do(reqA)
		if err != nil {
			t.Fatalf("tunnel list: %v", err)
		}
		defer respA.Body.Close()
		if respA.StatusCode == http.StatusUnauthorized {
			t.Fatalf("tunnel list with login token = 401")
		}
	}

	// 2. protected endpoint without token -> 401
	resp2, err := http.Post(srv.URL+"/api/client/list", "application/json", nil)
	if err != nil {
		t.Fatalf("client list: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/client/list without token = %d, want 401", resp2.StatusCode)
	}

	// 3. with valid JWT token -> 200 JSON envelope {code:0,message:ok,type:success,result:{total,items}}
	tok, err := utils.GenerateToken("admin")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/client/list", nil)
	req.Header.Set("Token", tok)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client list with token: %v", err)
	}
	defer resp3.Body.Close()
	b3, _ := io.ReadAll(resp3.Body)
	var env map[string]interface{}
	if err := json.Unmarshal(b3, &env); err != nil {
		t.Fatalf("client list non-JSON: %s", b3)
	}
	if env["code"] != float64(0) || env["type"] != "success" {
		t.Fatalf("client list envelope = %s", b3)
	}
	if _, ok := env["result"].(map[string]interface{}); !ok {
		t.Fatalf("client list result missing: %s", b3)
	}
	t.Logf("client list envelope: %s", b3)

	// 4. 静态资源：原版 4.9.3 只按文件精确服务，没有 SPA 回退（黑盒：
	//    GET /login 与 /index6us 都是 404；GET / → 302 /index.html）。
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse // 不跟随重定向，直接看 302
	}}
	rootReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	// web_basic_auth=true 时静态资源需要 Basic 凭证（原版行为）。
	rootReq.SetBasicAuth(cfg.WebUsername, cfg.WebPassword)
	respRoot, err := client.Do(rootReq)
	if err != nil {
		t.Fatalf("root: %v", err)
	}
	defer respRoot.Body.Close()
	if respRoot.StatusCode != http.StatusFound || respRoot.Header.Get("Location") != "/index.html" {
		t.Fatalf("GET / = %d (%q), want 302 → /index.html", respRoot.StatusCode, respRoot.Header.Get("Location"))
	}

	spaReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/index.html", nil)
	spaReq.SetBasicAuth(cfg.WebUsername, cfg.WebPassword)
	respSPA, err := client.Do(spaReq)
	if err != nil {
		t.Fatalf("spa: %v", err)
	}
	defer respSPA.Body.Close()
	if respSPA.StatusCode != http.StatusOK {
		t.Fatalf("GET /index.html = %d, want 200", respSPA.StatusCode)
	}
	b4, _ := io.ReadAll(respSPA.Body)
	if !strings.Contains(string(b4), "<html") {
		t.Fatalf("GET /index.html did not return the SPA shell")
	}

	// 未知路径与原版一致：404（不是 SPA 兜底、也不是 401）。
	resp404, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("unknown path: %v", err)
	}
	defer resp404.Body.Close()
	if resp404.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /login = %d, want 404 (original has no SPA fallback)", resp404.StatusCode)
	}
}
