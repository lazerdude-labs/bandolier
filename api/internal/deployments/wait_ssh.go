package deployments

import (
	"context"
	"fmt"
	"net"
	"time"
)

func waitForSSH(ctx context.Context, tfOutputs map[string]any, total, interval time.Duration) error {
	hosts := []string{}
	for _, k := range []string{"master_ip", "agent1_ip", "agent2_ip"} {
		if v, ok := tfOutputs[k].(string); ok && v != "" {
			hosts = append(hosts, net.JoinHostPort(v, "22"))
		}
	}
	if len(hosts) == 0 {
		return fmt.Errorf("no host outputs from terraform")
	}
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		all := true
		for _, h := range hosts {
			c, err := net.DialTimeout("tcp", h, 2*time.Second)
			if err != nil {
				all = false
				break
			}
			_ = c.Close()
		}
		if all {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("timed out waiting for ssh on %v", hosts)
}
