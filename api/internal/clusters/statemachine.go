package clusters

import "fmt"

type Status string

const (
	StatusPending      Status = "pending"
	StatusInitializing Status = "initializing"
	StatusInitialized  Status = "initialized"
	StatusDeploying    Status = "deploying"
	StatusReady        Status = "ready"
	StatusUpgrading    Status = "upgrading"
	StatusDegraded     Status = "degraded"
	StatusDestroying   Status = "destroying"
	StatusDestroyed    Status = "destroyed"
	StatusError        Status = "error"
)

var allowed = map[Status]map[Status]struct{}{
	StatusPending:      {StatusInitializing: {}},
	StatusInitializing: {StatusInitialized: {}, StatusError: {}},
	// `Initialized → Initializing` lets operators re-edit config that
	// hasn't been deployed yet (typo in the wizard, swap of network IPs,
	// etc.). Same idea for `Destroyed → Initializing` — change config
	// before redeploying. Live states (deploying, ready, upgrading,
	// destroying, degraded) intentionally do NOT permit re-init —
	// changing config of a running cluster requires destroy + redeploy
	// to avoid surprise drift between persisted config and live VMs.
	StatusInitialized: {StatusInitializing: {}, StatusDeploying: {}},
	StatusDeploying:   {StatusReady: {}, StatusError: {}},
	StatusReady:       {StatusUpgrading: {}, StatusDestroying: {}, StatusDegraded: {}},
	StatusUpgrading:   {StatusReady: {}, StatusError: {}},
	StatusDegraded:    {StatusDeploying: {}, StatusDestroying: {}},
	StatusDestroying:  {StatusDestroyed: {}, StatusError: {}},
	StatusDestroyed:   {StatusInitializing: {}, StatusDeploying: {}},
	StatusError:       {StatusInitializing: {}, StatusDeploying: {}, StatusDestroying: {}},
}

func CanTransition(from, to Status) error {
	if from == to {
		return nil
	}
	if next, ok := allowed[from]; ok {
		if _, ok := next[to]; ok {
			return nil
		}
	}
	return fmt.Errorf("cannot transition %s -> %s", from, to)
}
