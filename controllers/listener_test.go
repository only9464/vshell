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
