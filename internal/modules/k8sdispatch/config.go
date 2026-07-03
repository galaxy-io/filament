package k8sdispatch

import (
	"errors"
	"fmt"
	"os"
)

const defaultNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// Config controls the worker Job created for each run.
type Config struct {
	Namespace                   string
	WorkerImage                 string
	WorkerServiceAccount        string
	WorkerImagePullPolicy       string
	PersistenceDSN              string
	NATSURL                     string
	NATSStream                  string
	NATSSubjects                string
	JobNamePrefix               string
	BackoffLimit                int32
	TTLSecondsAfterFinished     *int32
	PassthroughEnv              []string
	Kubeconfig                  string
	WorkerRestartPolicy         string
	WorkerTerminationGraceSecs  *int64
	WorkerActiveDeadlineSeconds *int64
}

// ConfigFromEnv builds a Config for in-cluster use, with optional overrides.
func ConfigFromEnv() Config {
	cfg := Config{
		Namespace:             getenv("K8S_NAMESPACE", readFileTrim(defaultNamespaceFile)),
		WorkerImage:           os.Getenv("K8S_WORKER_IMAGE"),
		WorkerServiceAccount:  os.Getenv("K8S_WORKER_SERVICE_ACCOUNT"),
		WorkerImagePullPolicy: getenv("K8S_WORKER_IMAGE_PULL_POLICY", "IfNotPresent"),
		PersistenceDSN:        os.Getenv("PERSISTENCE_DSN"),
		NATSURL:               os.Getenv("NATS_URL"),
		NATSStream:            os.Getenv("NATS_STREAM"),
		NATSSubjects:          os.Getenv("NATS_SUBJECTS"),
		JobNamePrefix:         getenv("K8S_JOB_NAME_PREFIX", "filament"),
		BackoffLimit:          int32Env("K8S_JOB_BACKOFF_LIMIT", 1),
		Kubeconfig:            getenv("K8S_KUBECONFIG", os.Getenv("KUBECONFIG")),
		WorkerRestartPolicy:   getenv("K8S_WORKER_RESTART_POLICY", "Never"),
		PassthroughEnv:        splitCSV(os.Getenv("K8S_WORKER_PASSTHROUGH_ENV")),
	}
	if v := os.Getenv("K8S_JOB_TTL_SECONDS_AFTER_FINISHED"); v != "" {
		n := int32Env("K8S_JOB_TTL_SECONDS_AFTER_FINISHED", 0)
		cfg.TTLSecondsAfterFinished = &n
	}
	if v := os.Getenv("K8S_WORKER_TERMINATION_GRACE_SECONDS"); v != "" {
		n := int64Env("K8S_WORKER_TERMINATION_GRACE_SECONDS", 0)
		cfg.WorkerTerminationGraceSecs = &n
	}
	if v := os.Getenv("K8S_WORKER_ACTIVE_DEADLINE_SECONDS"); v != "" {
		n := int64Env("K8S_WORKER_ACTIVE_DEADLINE_SECONDS", 0)
		cfg.WorkerActiveDeadlineSeconds = &n
	}
	return cfg
}

func (c Config) validate() error {
	if c.Namespace == "" {
		return errors.New("k8sdispatch: namespace is required")
	}
	if c.WorkerImage == "" {
		return errors.New("k8sdispatch: worker image is required")
	}
	if c.PersistenceDSN == "" {
		return errors.New("k8sdispatch: control-plane dsn is required")
	}
	if c.NATSURL == "" {
		return errors.New("k8sdispatch: nats url is required")
	}
	if c.JobNamePrefix != "" && !isDNS1123Label(cleanDNS1123(c.JobNamePrefix)) {
		return fmt.Errorf("k8sdispatch: invalid job name prefix %q", c.JobNamePrefix)
	}
	return nil
}
