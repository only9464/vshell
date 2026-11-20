package c2engine

import (
	"testing"
	"time"
)

// 回归：tunnels/hosts 里 Health 未初始化时，巡检不得 panic（历史 bug：
// 只要存在一条隧道，后台巡检就会空指针打挂进程）。
func TestCheckTunnelAndHostWithNilHealth(t *testing.T) {
	hc := &HealthChecker{engine: GetEngine(), interval: 30 * time.Second, stopCh: make(chan struct{})}

	tunnel := &Tunnel{ID: 1, TargetAddr: "127.0.0.1:1"} // Health 为 nil
	hc.checkTunnel(tunnel)
	if tunnel.Health == nil {
		t.Fatalf("checkTunnel should initialise Health")
	}

	host := &Host{ID: 1, TargetStr: "127.0.0.1:1"}
	hc.checkHost(host)
	if host.Health == nil {
		t.Fatalf("checkHost should initialise Health")
	}
}
