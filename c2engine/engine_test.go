package c2engine

import "testing"

func TestEngineIsPubClientReturnsStoredClientStatus(t *testing.T) {
	e := &Engine{
		clients: map[int64]*Client{
			1: {ID: 1, Status: true},
			2: {ID: 2, Status: false},
		},
	}

	if !e.IsPubClient(1) {
		t.Fatalf("expected enabled client to be public")
	}
	if e.IsPubClient(2) {
		t.Fatalf("expected disabled client not to be public")
	}
	if e.IsPubClient(3) {
		t.Fatalf("expected missing client not to be public")
	}
}

// 在线筛选必须把"仅 Status=true、NowConn=0"的客户端算作在线
// （HTTP/WS 传输就是这样，此前只看 NowConn 会返回空）。
func TestGetClientListStatusFilterUsesStatus(t *testing.T) {
	e := &Engine{
		clients: map[int64]*Client{
			1: {ID: 1, Status: true, NowConn: 0, HostName: "online-by-status"},
			2: {ID: 2, Status: false, NowConn: 0, HostName: "offline"},
		},
		listeners: map[int64]*Listener{},
		tunnels:   map[int64]*Tunnel{},
		hosts:     map[int64]*Host{},
		tasks:     map[int64]*Task{},
	}
	list, total := e.GetClientList(0, 10, "Id", "ascend", "", "", 1)
	if total != 1 || len(list) != 1 || list[0].ID != 1 {
		t.Fatalf("status=1 → total=%d list=%v", total, list)
	}
	list, total = e.GetClientList(0, 10, "Id", "ascend", "", "", 2)
	if total != 1 || len(list) != 1 || list[0].ID != 2 {
		t.Fatalf("status=2 → total=%d list=%v", total, list)
	}
}

// 搜索必须能按主机名/用户名命中，且大小写不敏感，并支持首字母大写的排序字段。
func TestClientSearchAndSortFields(t *testing.T) {
	e := &Engine{
		clients: map[int64]*Client{
			1: {ID: 1, HostName: "Kali-01", UserName: "root", Status: true},
			2: {ID: 2, HostName: "win-box", UserName: "Administrator"},
		},
		listeners: map[int64]*Listener{},
		tunnels:   map[int64]*Tunnel{},
		hosts:     map[int64]*Host{},
		tasks:     map[int64]*Task{},
	}
	if list, total := e.GetClientList(0, 10, "Id", "ascend", "kali", "", 0); total != 1 || list[0].ID != 1 {
		t.Fatalf("search 'kali' → total=%d", total)
	}
	if list, total := e.GetClientList(0, 10, "Id", "ascend", "administrator", "", 0); total != 1 || list[0].ID != 2 {
		t.Fatalf("search 'administrator' → total=%d", total)
	}
	// 首字母大写的字段名也要能排序
	list, _ := e.GetClientList(0, 10, "HostName", "ascend", "", "", 0)
	if len(list) != 2 || list[0].HostName != "Kali-01" {
		t.Fatalf("sort by HostName failed: %v", list)
	}
}

// GetClientList 的 limit<=0 应视为"不限制"（概览页曾因此统计出 0 在线）。
func TestGetClientListLimitZeroMeansAll(t *testing.T) {
	e := &Engine{
		clients: map[int64]*Client{
			1: {ID: 1, Status: true},
			2: {ID: 2},
		},
		listeners: map[int64]*Listener{},
		tunnels:   map[int64]*Tunnel{},
		hosts:     map[int64]*Host{},
		tasks:     map[int64]*Task{},
	}
	list, total := e.GetClientList(0, 0, "", "", "", "", 0)
	if total != 2 || len(list) != 2 {
		t.Fatalf("limit=0 → total=%d len=%d（应为全部）", total, len(list))
	}
	if got := e.OnlineClientCount(); got != 1 {
		t.Fatalf("OnlineClientCount=%d（应为 1）", got)
	}
}
