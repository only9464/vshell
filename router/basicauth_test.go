package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"vshell/utils"
)

// Basic Auth 的行为按原版 4.9.3 黑盒对齐（docs/REFERENCE.md R-13）：
//
//	GET /                无凭证 → 401 + WWW-Authenticate: Basic realm="Authorization Required"
//	                              body "401 Unauthorized\n"
//	GET /index.html      无凭证 → 401；带正确凭证 → 200
//	GET /api/*           不受 Basic 保护（靠 Token 鉴权）
//	GET /login 等未知路径 → 404（不触发 401）
func TestBasicAuthMatchesOriginal(t *testing.T) {
	cfg := utils.GetFullSettings()
	prevAuth := cfg.WebBasicAuth
	cfg.WebBasicAuth = true
	t.Cleanup(func() { cfg.WebBasicAuth = prevAuth })

	// 静态文件必须在 cwd 下可见（与生产一致）。
	if _, err := os.Stat("static/index.html"); err != nil {
		if _, err := os.Stat("../static/index.html"); err == nil {
			_ = os.Chdir("..")
		} else {
			t.Skip("static/index.html 不可见，跳过 Basic Auth 用例")
		}
	}

	mux := InitRouter()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("root_without_credentials", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("get /: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="Authorization Required"` {
			t.Fatalf("WWW-Authenticate = %q", got)
		}
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "401 Unauthorized\n" {
			t.Fatalf("body = %q, want %q", body, "401 Unauthorized\n")
		}
	})

	t.Run("root_with_credentials_redirects", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
		req.SetBasicAuth(cfg.WebUsername, cfg.WebPassword)
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("get /: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/index.html" {
			t.Fatalf("status = %d location = %q, want 302 → /index.html",
				resp.StatusCode, resp.Header.Get("Location"))
		}
	})

	t.Run("index_html_requires_credentials", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/index.html")
		if err != nil {
			t.Fatalf("get index: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("api_not_gated_by_basic_auth", func(t *testing.T) {
		resp, err := http.Post(srv.URL+"/api/login", "application/json",
			strings.NewReader(`{"username":"x","password":"y"}`))
		if err != nil {
			t.Fatalf("post login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			// 401 只应来自 Token 校验；/api/login 是公开端点，必须是 200 + 业务错误。
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("/api/login was gated by basic auth (401): %s", body)
		}
	})

	t.Run("unknown_path_is_404_not_401", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/login")
		if err != nil {
			t.Fatalf("get /login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (original has no SPA fallback)", resp.StatusCode)
		}
	})
}

// 新版 Vue3 面板（/app/）也受 basic auth 保护，且未认证时是 401 而不是 404。
func TestVueAppRequiresBasicAuth(t *testing.T) {
	cfg := utils.GetFullSettings()
	prev := cfg.WebBasicAuth
	cfg.WebBasicAuth = true
	t.Cleanup(func() { cfg.WebBasicAuth = prev })

	if _, err := os.Stat("web/dist/index.html"); err != nil {
		if _, err2 := os.Stat("../web/dist/index.html"); err2 == nil {
			_ = os.Chdir("..")
		} else {
			t.Skip("web/dist 未构建（pnpm build），跳过")
		}
	}

	srv := httptest.NewServer(InitRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/app/")
	if err != nil {
		t.Fatalf("get /app/: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/app/ without credentials = %d, want 401", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/app/", nil)
	req.SetBasicAuth(cfg.WebUsername, cfg.WebPassword)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authed get: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/app/ with credentials = %d, want 200", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "<div id=\"app\">") {
		t.Fatalf("/app/ did not serve the Vue shell: %q", string(body[:min(len(body), 200)]))
	}
}
