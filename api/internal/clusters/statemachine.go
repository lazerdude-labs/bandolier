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
	StatusInitialized:  {StatusDeploying: {}},
	StatusDeploying:    {StatusReady: {}, StatusError: {}},
	StatusReady:        {StatusUpgrading: {}, StatusDestroying: {}, StatusDegraded: {}},
	StatusUpgrading:    {StatusReady: {}, StatusError: {}},
	StatusDegraded:     {StatusDeploying: {}, StatusDestroying: {}},
	StatusDestroying:   {StatusDestroyed: {}, StatusError: {}},
	StatusDestroyed:    {StatusDeploying: {}},
	StatusError:        {StatusInitializing: {}, StatusDeploying: {}, StatusDestroying: {}},
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
