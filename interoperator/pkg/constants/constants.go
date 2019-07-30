package constants

import "time"

// Constants used by interoperator
const (
	LeaderElectionID = "interoperator-leader-election-helper"
	FinalizerName    = "interoperator.servicefabrik.io"
	ErrorCountKey    = "interoperator.servicefabrik.io/error"
	LastOperationKey = "interoperator.servicefabrik.io/lastoperation"
	ErrorThreshold   = 10

	ConfigMapName   = "interoperator-config"
	ConfigMapKey    = "config"
	NamespaceEnvKey = "POD_NAMESPACE"

	DefaultServiceFabrikNamespace = "default"
	DefaultInstanceWorkerCount    = 10
	DefaultBindingWorkerCount     = 20
	DefaultSchedulerWorkerCount   = 10

	PlanWatchDrainTimeout = time.Second * 2
)
