package telemetry

import (
	"testing"
)

const sampleNodesJSON = `{
  "items": [
    {
      "metadata": {"name": "master1"},
      "status": {
        "addresses": [{"type": "InternalIP", "address": "192.0.2.21"}],
        "nodeInfo": {"kubeletVersion": "v1.31.12+k3s1"},
        "conditions": [
          {"type": "Ready", "status": "True", "lastHeartbeatTime": "2026-05-02T10:00:00Z"}
        ]
      }
    },
    {
      "metadata": {"name": "worker1"},
      "status": {
        "addresses": [{"type": "InternalIP", "address": "192.0.2.22"}],
        "nodeInfo": {"kubeletVersion": "v1.31.12+k3s1"},
        "conditions": [
          {"type": "Ready", "status": "True", "lastHeartbeatTime": "2026-05-02T10:00:01Z"}
        ]
      }
    }
  ]
}`

func TestParseKubeNodes(t *testing.T) {
	got, err := parseKubeNodes([]byte(sampleNodesJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(got))
	}
	if got[0].Name != "master1" || got[0].IP != "192.0.2.21" {
		t.Fatalf("master1 mismatch: %+v", got[0])
	}
	if got[0].K3sVersion == nil || *got[0].K3sVersion != "v1.31.12+k3s1" {
		t.Fatalf("master1 version: %+v", got[0].K3sVersion)
	}
	if got[0].Ready == nil || !*got[0].Ready {
		t.Fatalf("master1 ready: %+v", got[0].Ready)
	}
}
