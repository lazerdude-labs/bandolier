package deployments

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/lazerdude-labs/bandolier/api/internal/ansible"
	"github.com/lazerdude-labs/bandolier/api/internal/audit"
	"github.com/lazerdude-labs/bandolier/api/internal/clusters"
	"github.com/lazerdude-labs/bandolier/api/internal/profiles"
	"github.com/lazerdude-labs/bandolier/api/internal/store"
)

// Upgrade kicks off an in-place k3s version upgrade for the given cluster. It
// returns the deployment ID immediately and runs the upgrade playbook in a
// goroutine. No terraform is involved — only the existing cluster's nodes are
// upgraded via Ansible.
func (e *Executor) Upgrade(ctx context.Context, clusterID, k3sVersion string) (string, error) {
	c, err := e.Store.GetCluster(ctx, clusterID)
	if err != nil {
		return "", err
	}
	if err := clusters.CanTransition(clusters.Status(c.Status), clusters.StatusUpgrading); err != nil {
		return "", clusters.ErrInvalidTransition
	}
	prof, err := e.Registry.Get(c.Profile)
	if err != nil {
		return "", err
	}
	if !e.Mutex.TryLock(clusterID) {
		return "", clusters.ErrLocked
	}

	depID := NewDeploymentID()
	logPath := filepath.Join(e.LogRoot, depID+".log")
	_ = os.MkdirAll(e.LogRoot, 0o755)
	logFile, err := os.Create(logPath)
	if err != nil {
		e.Mutex.Unlock(clusterID)
		return "", err
	}

	actorID := actorIDFromCtx(ctx)
	if err := e.Store.CreateDeployment(ctx, &store.Deployment{
		ID:        depID,
		ClusterID: clusterID,
		Operation: "upgrade",
		Status:    "running",
		LogPath:   sql.NullString{String: logPath, Valid: true},
		ActorID:   actorIDPtr(actorID),
	}); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}
	if err := e.Store.UpdateClusterStatus(ctx, clusterID, string(clusters.StatusUpgrading)); err != nil {
		_ = logFile.Close()
		e.Mutex.Unlock(clusterID)
		return "", err
	}
	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterUpgrade),
		Target:  clusterID,
		Outcome: audit.OutcomeStarted,
		Details: map[string]any{"k3s_version": k3sVersion},
	})

	runCtx, cancelRun := context.WithCancel(context.Background())
	e.register(depID, cancelRun)
	go e.runUpgrade(runCtx, depID, clusterID, k3sVersion, prof, logFile, actorID)
	return depID, nil
}

func (e *Executor) runUpgrade(ctx context.Context, depID, clusterID, k3sVersion string, prof profiles.Profile, logFile *os.File, actorID int64) {
	defer logFile.Close()
	defer e.deregister(depID)
	defer e.Mutex.Unlock(clusterID)

	publish := func(ev Event) { e.Hub.Publish(depID, ev) }
	logWriter := teeWriter{file: logFile, hub: e.Hub, depID: depID}

	fail := func(msg string) {
		bg := context.Background()
		status, outcome := finishStatus(ctx.Err(), true)
		_ = e.Store.FinishDeployment(bg, depID, status, msg)
		_ = e.Store.UpdateClusterStatus(bg, clusterID, string(clusters.StatusError))
		_, _ = audit.Write(bg, e.Store, audit.Entry{
			ActorID: actorID,
			Action:  string(audit.ActionClusterUpgrade),
			Target:  clusterID,
			Outcome: outcome,
			Details: map[string]any{"k3s_version": k3sVersion, "error": msg},
		})
		publish(Event{Type: EventDeploymentComplete, Status: status, Text: msg})
	}

	// Build a private_data_dir for ansible-runner. Upgrade does not touch
	// terraform — we only need the inventory + extra-vars to invoke the
	// upgrade playbook.
	runDir, err := os.MkdirTemp("", "upgrade-*")
	if err != nil {
		fail("temp dir: " + err.Error())
		return
	}
	defer os.RemoveAll(runDir)

	// Homelab BuildInventory ignores tfOutputs and reads the vault network/SSH
	// secrets directly. Pass nil so we don't need to re-run terraform output.
	inv, err := prof.BuildInventory(ctx, clusterID, nil, runDir, e.Vault)
	if err != nil {
		fail("build inventory: " + err.Error())
		return
	}
	extraVars, err := prof.BuildUpgradeVars(ctx, clusterID, k3sVersion, e.Vault)
	if err != nil {
		fail("build upgrade vars: " + err.Error())
		return
	}

	publish(Event{Type: EventStepStart, Step: "ansible.upgrade"})

	runner := ansible.New(e.AnsibleBin, runDir)
	if err := runner.Prepare(prof.AnsiblePlaybookDir(), inv, extraVars); err != nil {
		publish(Event{Type: EventStepEnd, Step: "ansible.upgrade", Exit: 1})
		fail("ansible prepare: " + err.Error())
		return
	}

	onEvent := func(ev ansible.Event) {
		publish(Event{Type: EventAnsible, Data: ev})
	}
	// Hardcoded relative path: the upgrade playbook lives alongside setup.yml
	// inside the project/ root that ansible-runner Prepare populates.
	const playbookFile = "playbooks/upgrade.yml"
	if err := runner.Run(ctx, playbookFile, onEvent, &logWriter, &logWriter); err != nil {
		publish(Event{Type: EventStepEnd, Step: "ansible.upgrade", Exit: 1})
		fail("ansible run: " + err.Error())
		return
	}
	publish(Event{Type: EventStepEnd, Step: "ansible.upgrade", Exit: 0})

	_ = e.Store.FinishDeployment(ctx, depID, "succeeded", "")
	_ = e.Store.UpdateClusterStatus(ctx, clusterID, string(clusters.StatusReady))
	_, _ = audit.Write(ctx, e.Store, audit.Entry{
		ActorID: actorID,
		Action:  string(audit.ActionClusterUpgrade),
		Target:  clusterID,
		Outcome: audit.OutcomeSucceeded,
		Details: map[string]any{"k3s_version": k3sVersion},
	})
	publish(Event{Type: EventDeploymentComplete, Status: "succeeded"})
}
