package controllers

import (
	"strings"
	"testing"
)

// 黑盒对齐（R15）：/listener/edit 读**大写** Id，缺 Id 时官方返回 "id is null"。
func TestListenerEditRequiresCapitalID(t *testing.T) {
	base, rec := newTestController()
	base.Controller.Ctx.Input.RequestBody = []byte(`{"id":1,"Mode":"tcp","Vkey":"k"}`)
	c := &ListenerController{ApiBaseController: *base}
	c.Edit()
	if body := rec.Body.String(); !strings.Contains(body, "id is null") {
		t.Fatalf("lowercase id should be rejected with \"id is null\", got %s", body)
	}
}

// 黑盒对齐（R40）：listener/stop 对不存在的 id 返回 "not found"。
func TestListenerStopMissingReturnsNotFound(t *testing.T) {
	base, rec := newTestController()
	base.Controller.Ctx.Input.RequestBody = []byte(`{"id":987654}`)
	c := &ListenerController{ApiBaseController: *base}
	c.Stop()
	if body := rec.Body.String(); !strings.Contains(body, "not found") {
		t.Fatalf("stop missing → %s", body)
	}
}
