package c2engine

import "testing"

// 原生 TCP 监听器的载荷必须写 tcp:// 前缀，否则 agent 用 http 连裸 TCP 端口。
func TestPoServerURLTCP(t *testing.T) {
	if got := poServerURL("tcp", "1.2.3.4:8084"); got != "tcp://1.2.3.4:8084" {
		t.Fatalf("poServerURL(tcp) = %q", got)
	}
	if got := poServerURL("raw", "1.2.3.4:8084"); got != "tcp://1.2.3.4:8084" {
		t.Fatalf("poServerURL(raw) = %q", got)
	}
	if got := poServerURL("http", "1.2.3.4:8082"); got != "http://1.2.3.4:8082" {
		t.Fatalf("poServerURL(http) = %q", got)
	}
}
