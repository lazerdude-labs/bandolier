package deployments_test

import (
	"testing"

	"github.com/lazerdude-labs/bandolier/api/internal/deployments"
)

func TestNewDeploymentID(t *testing.T) {
	a := deployments.NewDeploymentID()
	b := deployments.NewDeploymentID()
	if a == b || len(a) != 32 {
		t.Fatalf("ids not unique or wrong length: %s %s", a, b)
	}
}
