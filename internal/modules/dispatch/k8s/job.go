package k8s

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/galaxy-io/filament"
)

func (m *Module) jobForSpec(spec filament.RunSpec) *batchv1.Job {
	name := jobName(m.cfg.JobNamePrefix, spec.Run)
	// PERSISTENCE_DSN, NATS_URL, and ENCRYPTION_KEY arrive via the worker
	// Secret (envFrom); only per-run and plain config are set explicitly.
	envFrom := []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: m.cfg.WorkerSecretName}},
	}}
	env := []corev1.EnvVar{{Name: "RUN_ID", Value: string(spec.Run)}}
	if m.cfg.NATSStream != "" {
		env = append(env, corev1.EnvVar{Name: "NATS_STREAM", Value: m.cfg.NATSStream})
	}
	if m.cfg.NATSSubjects != "" {
		env = append(env, corev1.EnvVar{Name: "NATS_SUBJECTS", Value: m.cfg.NATSSubjects})
	}
	if m.cfg.SecretProvider != "" {
		env = append(env, corev1.EnvVar{Name: "SECRET_PROVIDER", Value: m.cfg.SecretProvider})
	}
	if m.cfg.EncryptionKeyID != "" {
		env = append(env, corev1.EnvVar{Name: "ENCRYPTION_KEY_ID", Value: m.cfg.EncryptionKeyID})
	}
	if m.cfg.SecretsPrefix != "" {
		env = append(env, corev1.EnvVar{Name: "SECRETS_PREFIX", Value: m.cfg.SecretsPrefix})
	}
	if m.cfg.AWSRegion != "" {
		env = append(env, corev1.EnvVar{Name: "AWS_REGION", Value: m.cfg.AWSRegion})
	}
	if m.cfg.OTELEndpoint != "" {
		env = append(env, corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: m.cfg.OTELEndpoint})
	}
	if m.cfg.OTELProtocol != "" {
		env = append(env, corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_PROTOCOL", Value: m.cfg.OTELProtocol})
	}

	restartPolicy := m.cfg.WorkerRestartPolicy
	if restartPolicy == "" {
		restartPolicy = "Never"
	}

	podSpec := corev1.PodSpec{
		RestartPolicy:      corev1.RestartPolicy(restartPolicy),
		ServiceAccountName: m.cfg.WorkerServiceAccount,
		Containers: []corev1.Container{{
			Name:            "worker",
			Image:           m.cfg.WorkerImage,
			ImagePullPolicy: corev1.PullPolicy(m.cfg.WorkerImagePullPolicy),
			Env:             env,
			EnvFrom:         envFrom,
		}},
		TerminationGracePeriodSeconds: m.cfg.WorkerTerminationGraceSecs,
		ActiveDeadlineSeconds:         m.cfg.WorkerActiveDeadlineSeconds,
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "filament",
				"app.kubernetes.io/component": "worker",
				"filament.galaxy.io/run-id":   string(spec.Run),
				"filament.galaxy.io/tenant":   string(spec.Tenant),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &m.cfg.BackoffLimit,
			TTLSecondsAfterFinished: m.cfg.TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":      "filament",
						"app.kubernetes.io/component": "worker",
						"filament.galaxy.io/run-id":   string(spec.Run),
					},
				},
				Spec: podSpec,
			},
		},
	}
}
