package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"vshell/utils"

	"github.com/beego/beego/v2/server/web/context"
)

// 契约测试：把本实现的响应与原版 4.9.3 的黑盒抓包（testdata/original/*.json，
// 由 scripts/probe-original.sh 采集）逐项比对。
//
// 比对内容刻意只覆盖"形状"：
//   - 对象键集合（排序后逐字比较）
//   - 空列表必须序列化成 null（原版如此）
//   - 错误文案逐字比较
//
// 不比对数值：cpu/disk/io_recv 这类指标随机器与时刻变化，键集合一致才是契约。
//
// 之所以需要这层测试：单测只验证"我按我想的实现"，这里验证"我按原版实现"。

const originalFixtures = "../testdata/original"

// invokeJSON 在给定请求上执行一个控制器方法，并把 JSON 响应解析成 map。
func invokeJSON(t *testing.T, method, path, body string, fn func(ctx *context.Context)) map[string]interface{} {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	ctx := context.NewContext()
	ctx.Reset(rec, req)
	if body != "" {
		ctx.Input.RequestBody = []byte(body)
	}
	fn(ctx)
	var obj map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("%s: response is not JSON: %q", path, rec.Body.String())
	}
	return obj
}

// loadFixture 读取原版抓包。
func loadFixture(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(originalFixtures, name+".json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v (先跑 scripts/probe-original.sh)", name, err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return obj
}

// assertSameKeys 断言 got 与 want 的键集合完全一致。
func assertSameKeys(t *testing.T, label string, got, want map[string]interface{}) {
	t.Helper()
	keys := func(m map[string]interface{}) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	g, w := keys(got), keys(want)
	if len(g) == len(w) {
		same := true
		for i := range g {
			if g[i] != w[i] {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	t.Fatalf("%s: key set mismatch\n got(%d)=%v\nwant(%d)=%v", label, len(g), g, len(w), w)
}

// resultOf 取响应里的 result 对象。
func resultOf(t *testing.T, label string, obj map[string]interface{}) map[string]interface{} {
	t.Helper()
	r, ok := obj["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s: result is not an object: %v", label, obj)
	}
	return r
}

// TestContractDashboardKeys 比对 /api/dashboard/info 的键集合（原版 39 个键）。
func TestContractDashboardKeys(t *testing.T) {
	initEngineStorage(t)
	want := resultOf(t, "fixture", loadFixture(t, "dashboard"))

	got := invokeJSON(t, http.MethodGet, "/api/dashboard/info", "", func(ctx *context.Context) {
		c := &DashboardController{}
		c.Ctx = ctx
		c.Data = map[interface{}]interface{}{}
		c.Info()
	})
	assertSameKeys(t, "dashboard.result", resultOf(t, "dashboard", got), want)
}

// TestContractListShapes 比对 client/list、listener/list、tunnel/list。
func TestContractListShapes(t *testing.T) {
	initEngineStorage(t)

	t.Run("client_list", func(t *testing.T) {
		want := resultOf(t, "fixture", loadFixture(t, "client_list"))
		got := resultOf(t, "client_list", invokeJSON(t, http.MethodPost, "/api/client/list",
			`{"page":1,"pageSize":10}`, func(ctx *context.Context) {
				c := &ClientController{}
				c.Ctx = ctx
				c.Data = map[interface{}]interface{}{}
				c.List()
			}))
		assertSameKeys(t, "client_list.result", got, want)
	})

	t.Run("listener_list", func(t *testing.T) {
		want := resultOf(t, "fixture", loadFixture(t, "listener_list"))
		got := resultOf(t, "listener_list", invokeJSON(t, http.MethodPost, "/api/listener/list",
			`{"page":1,"pageSize":10}`, func(ctx *context.Context) {
				c := &ListenerController{}
				c.Ctx = ctx
				c.Data = map[interface{}]interface{}{}
				c.List()
			}))
		assertSameKeys(t, "listener_list.result", got, want)
		// 原版空列表是 null，不是 []。
		if got["items"] != nil {
			t.Fatalf("listener/list empty items = %#v, want null", got["items"])
		}
	})

	t.Run("tunnel_list", func(t *testing.T) {
		want := resultOf(t, "fixture", loadFixture(t, "tunnel_list"))
		got := resultOf(t, "tunnel_list", invokeJSON(t, http.MethodPost, "/api/tunnel/list",
			`{"page":1,"pageSize":10}`, func(ctx *context.Context) {
				c := &TunnelController{}
				c.Ctx = ctx
				c.Data = map[interface{}]interface{}{}
				c.List()
			}))
		assertSameKeys(t, "tunnel_list.result", got, want)
		if got["items"] != nil {
			t.Fatalf("tunnel/list empty items = %#v, want null", got["items"])
		}
	})

	t.Run("listener_items_keys", func(t *testing.T) {
		// 原版每项 14 个键（不含 Port/Host/Proxy/Salt/vip）；建一条监听器后比对。
		want := loadFixture(t, "listener_list")["result"].(map[string]interface{})
		_ = want
		id := newTestListener(t, "127.0.0.1:0", "", "http", "vk", "")
		got := resultOf(t, "listener_list", invokeJSON(t, http.MethodPost, "/api/listener/list",
			`{"page":1,"pageSize":10}`, func(ctx *context.Context) {
				c := &ListenerController{}
				c.Ctx = ctx
				c.Data = map[interface{}]interface{}{}
				c.List()
			}))
		items, ok := got["items"].([]interface{})
		if !ok || len(items) == 0 {
			t.Fatalf("listener/list items = %#v, want a non-empty array (id=%d)", got["items"], id)
		}
		first := items[0].(map[string]interface{})
		wantKeys := []string{
			"ConnectAddr", "DNSDomain", "DisconnectTimeout", "EncryptSalt", "Id",
			"ListenAddr", "MaxDNSsize", "Mode", "OssUrl", "PingInterval",
			"PublicDNS", "Remark", "Status", "Vkey",
		}
		assertSameKeys(t, "listener item", first, func() map[string]interface{} {
			m := make(map[string]interface{}, len(wantKeys))
			for _, k := range wantKeys {
				m[k] = nil
			}
			return m
		}())
	})
}

// TestContractRunnerList 比对插件列表形状：[{id,name}]。
func TestContractRunnerList(t *testing.T) {
	initEngineStorage(t)
	wantList, ok := loadFixture(t, "runner_list")["result"].([]interface{})
	if !ok || len(wantList) == 0 {
		t.Fatalf("fixture runner_list.result should be a non-empty array")
	}

	got := invokeJSON(t, http.MethodGet, "/api/runner/list", "", func(ctx *context.Context) {
		c := &RunnerController{}
		c.Ctx = ctx
		c.Data = map[interface{}]interface{}{}
		c.List()
	})
	gotList, ok := got["result"].([]interface{})
	if !ok {
		t.Fatalf("runner/list result = %#v, want array", got["result"])
	}
	if len(gotList) == 0 {
		t.Skip("本机 plugins/ 为空，跳过条目形状比对")
	}
	assertSameKeys(t, "runner/list[0]", gotList[0].(map[string]interface{}), wantList[0].(map[string]interface{}))
}

// TestContractSettingGet 比对 /api/setting/get 的三个键。
func TestContractSettingGet(t *testing.T) {
	initEngineStorage(t)
	want := resultOf(t, "fixture", loadFixture(t, "setting_get"))
	got := invokeJSON(t, http.MethodGet, "/api/setting/get", "", func(ctx *context.Context) {
		c := &SettingController{}
		c.Ctx = ctx
		c.Data = map[interface{}]interface{}{}
		c.Get()
	})
	assertSameKeys(t, "setting/get.result", resultOf(t, "setting/get", got), want)
}

// TestContractErrorMessages 逐字比对错误文案（面板与用户直接看到的就是这些）。
func TestContractErrorMessages(t *testing.T) {
	initEngineStorage(t)

	t.Run("screenshot_missing", func(t *testing.T) {
		want := loadFixture(t, "screenshot_missing")["message"]
		got := invokeJSON(t, http.MethodGet, "/api/screenshot/get?id=1&quality=normal", "", func(ctx *context.Context) {
			c := &ScreenshotController{}
			c.Ctx = ctx
			c.Data = map[interface{}]interface{}{}
			c.Get()
		})
		if got["message"] != want {
			t.Fatalf("message = %v, want %v", got["message"], want)
		}
	})

	t.Run("getdisk_missing", func(t *testing.T) {
		want := loadFixture(t, "getdisk_missing")["message"]
		got := invokeJSON(t, http.MethodPost, "/api/file/getdisk", `{"id":1}`, func(ctx *context.Context) {
			c := &FileController{}
			c.Ctx = ctx
			c.Data = map[interface{}]interface{}{}
			c.Getdisk()
		})
		if got["message"] != want {
			t.Fatalf("message = %v, want %v", got["message"], want)
		}
	})

	t.Run("terminal_missing", func(t *testing.T) {
		want := loadFixture(t, "terminal_missing")["message"]
		got := invokeJSON(t, http.MethodPost, "/api/terminal/shell", `{"id":1,"command":"id"}`, func(ctx *context.Context) {
			c := &TerminalController{}
			c.Ctx = ctx
			c.Data = map[interface{}]interface{}{}
			c.Shell()
		})
		if got["message"] != want {
			t.Fatalf("message = %v, want %v", got["message"], want)
		}
	})

	t.Run("install_missing", func(t *testing.T) {
		want := loadFixture(t, "install_missing")["message"]
		got := invokeJSON(t, http.MethodPost, "/api/install/install", `{"id":1,"name":"x"}`, func(ctx *context.Context) {
			c := &InstallController{}
			c.Ctx = ctx
			c.Data = map[interface{}]interface{}{}
			c.Install()
		})
		if got["message"] != want {
			t.Fatalf("message = %v, want %v", got["message"], want)
		}
	})

	t.Run("download_stage_not_found", func(t *testing.T) {
		want := loadFixture(t, "download_stage")["message"]
		got := invokeJSON(t, http.MethodGet, "/api/download/stage?id=1&arch=windows_amd64", "", func(ctx *context.Context) {
			c := &DownloadController{}
			c.Ctx = ctx
			c.Data = map[interface{}]interface{}{}
			c.Stage()
		})
		if got["message"] != want {
			t.Fatalf("message = %v, want %v", got["message"], want)
		}
	})
}

// TestContractLoginEnvelope 校验登录信封与 result 键集合。
func TestContractLoginEnvelope(t *testing.T) {
	initEngineStorage(t)
	want := loadFixture(t, "login")

	cfg := utils.GetFullSettings()
	body := `{"username":"` + cfg.WebUsername + `","password":"` + cfg.WebPassword + `"}`
	got := invokeJSON(t, http.MethodPost, "/api/login", body, func(ctx *context.Context) {
		c := &LoginController{}
		c.Ctx = ctx
		c.Data = map[interface{}]interface{}{}
		c.Login()
	})

	assertSameKeys(t, "login envelope", got, want)
	if res, ok := got["result"].(map[string]interface{}); ok {
		assertSameKeys(t, "login.result", res, want["result"].(map[string]interface{}))
	} else {
		t.Fatalf("login result = %#v, want an object", got["result"])
	}
}

// 设置保存后必须立刻能在 Get 里读到（不需要重启）。
func TestSettingEditVisibleImmediately(t *testing.T) {
	dir := t.TempDir()
	prev := settingConfPathFn
	settingConfPathFn = func() string { return dir + "/setting.conf" }
	t.Cleanup(func() { settingConfPathFn = prev })

	base, rec := newTestController()
	base.Controller.Ctx.Input.RequestBody = []byte(`{"wx_key":"k1","dingding_access_token":"t1","dingding_key_word":"w1"}`)
	(&SettingController{ApiBaseController: *base}).Edit()
	if body := rec.Body.String(); !strings.Contains(body, "code\":0") {
		t.Fatalf("edit failed: %s", body)
	}

	base2, rec2 := newTestController()
	(&SettingController{ApiBaseController: *base2}).Get()
	if body := rec2.Body.String(); !strings.Contains(body, "k1") {
		t.Fatalf("get did not see the new value immediately: %s", body)
	}
}
