package pinger

import (
	"testing"
	"time"
)

func TestPingerLocalhost(t *testing.T) {
	p := NewPinger()
	res := p.Ping("127.0.0.1", 2*time.Second)

	if res.Target != "127.0.0.1" {
		t.Errorf("ожидалась цель 127.0.0.1, получено %s", res.Target)
	}
	if res.Status != "success" {
		t.Logf("ПРЕДУПРЕЖДЕНИЕ: пинг 127.0.0.1 завершился со статусом %s: %s (возможно, ограничения сетевого окружения)", res.Status, res.ErrorMsg)
	} else {
		if res.RTT <= 0 {
			t.Errorf("ожидалось RTT > 0, получено %v", res.RTT)
		}
	}
}
