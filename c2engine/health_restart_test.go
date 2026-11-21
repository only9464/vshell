package c2engine

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// 健康检查连续失败 → 数据面被重启（端口仍可连接）。
func TestHealthFailureRestartsTunnel(t *testing.T) {
	probe, _ := net.Listen("tcp", "127.0.0.1:0")
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	ts, err := StartTunnelServer(7001, port)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer StopTunnelServer(7001)
	opened := 0
	ts.SetOnOpen(func(uint32) error { opened++; return nil })

	tunnel := &Tunnel{ID: 7001, Port: port, RunStatus: true, Health: NewHealth()}
	tunnel.Health.CheckType = "tcp"
	tunnel.Health.MaxFail = 1
	tunnel.Health.RecordFail()

	hc := &HealthChecker{engine: GetEngine(), interval: time.Second, stopCh: make(chan struct{})}
	hc.checkTunnel(tunnel)

	if !tunnel.RunStatus {
		t.Fatalf("tunnel should be running after restart")
	}
	newTS := GetTunnelServer(7001)
	if newTS == nil || newTS == ts {
		t.Fatalf("tunnel data plane was not restarted")
	}
	if !newTS.hasOnOpen() {
		t.Fatalf("OnOpen callback lost after restart")
	}
	// 重启后端口仍然可连
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		t.Fatalf("port not listening after restart: %v", err)
	}
	_ = conn.Close()
}
