package k8s

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
	NATSStream                  string
	NATSSubjects                string
	JobNamePrefix               string
	BackoffLimit                int32
	TTLSecondsAfterFinished     *int32
	WorkerSecretName            string
	SecretProvider              string
	EncryptionKeyID             string
	SecretsPrefix               string
	AWSRegion                   string
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
		NATSStream:            os.Getenv("NATS_STREAM"),
		NATSSubjects:          os.Getenv("NATS_SUBJECTS"),
		JobNamePrefix:         getenv("K8S_JOB_NAME_PREFIX", "filament"),
		BackoffLimit:          int32Env("K8S_JOB_BACKOFF_LIMIT", 1),
		Kubeconfig:            getenv("K8S_KUBECONFIG", os.Getenv("KUBECONFIG")),
		WorkerRestartPolicy:   getenv("K8S_WORKER_RESTART_POLICY", "Never"),
		SecretProvider:        os.Getenv("SECRET_PROVIDER"),
		EncryptionKeyID:       os.Getenv("ENCRYPTION_KEY_ID"),
		SecretsPrefix:         os.Getenv("SECRETS_PREFIX"),
		AWSRegion:             os.Getenv("AWS_REGION"),
		// envFrom'd whole into worker pods: carries PERSISTENCE_DSN, NATS_URL,
		// and ENCRYPTION_KEY under keys named after the env vars they feed.
		WorkerSecretName: os.Getenv("K8S_WORKER_SECRET_NAME"),
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
	if c.JobNamePrefix != "" && !isDNS1123Label(cleanDNS1123(c.JobNamePrefix)) {
		return fmt.Errorf("k8sdispatch: invalid job name prefix %q", c.JobNamePrefix)
	}
	if c.WorkerSecretName == "" {
		return errors.New("k8sdispatch: worker secret is required")
	}
	return nil
}
