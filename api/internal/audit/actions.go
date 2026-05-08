package audit

// Action enumerates every audit-logged event type. Constants live here so
// callers can't typo an action name and silently miss the audit row.
type Action string

const (
	ActionAuthSetup      Action = "auth_setup"
	ActionAuthLogin      Action = "auth_login"
	ActionAuthLogout     Action = "auth_logout"
	ActionChangePassword Action = "change_password"
	ActionClusterCreate              Action = "cluster_create"
	ActionClusterInit                Action = "cluster_initialize"
	ActionClusterDeploy              Action = "cluster_deploy"
	ActionClusterDestroy             Action = "cluster_destroy"
	ActionClusterUpgrade             Action = "cluster_upgrade"
	ActionClusterKubeconfigDownload  Action = "cluster_kubeconfig_download"
	ActionClusterJoinTokenRetrieve   Action = "cluster_join_token_retrieve"
	ActionAppRepoAdd    Action = "app_repo_add"
	ActionAppRepoRemove Action = "app_repo_remove"
	ActionAppInstall    Action = "app_install"
	ActionAppUpgrade   Action = "app_upgrade"
	ActionAppUninstall Action = "app_uninstall"

	ActionAppBundleInstall Action = "app_bundle_install"
	ActionClusterDNSWrite  Action = "cluster_dns_write"
	ActionClusterCertIssue Action = "cluster_cert_issue"
	ActionClusterCertRenew Action = "cluster_cert_renew"

	ActionVaultTokenRenew Action = "vault_token_renew"
)

// Outcome values for long-running ops that emit a started + terminal pair.
// One-shot ops continue to use OutcomeSuccess / OutcomeFailure (defined in
// log.go) — pre-existing rows stay valid under the widened CHECK constraint.
const (
	OutcomeStarted   Outcome = "started"
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeCancelled Outcome = "cancelled"
)
