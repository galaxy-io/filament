package k8sdispatch

import (
	"os"

	ingestion "github.com/galaxy-io/filament"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *Module) jobForSpec(spec ingestion.RunSpec) *batchv1.Job {
	name := jobName(m.cfg.JobNamePrefix, spec.Run)
	env := []corev1.EnvVar{
		{Name: "RUN_ID", Value: string(spec.Run)},
		{Name: "CONTROL_PLANE_DSN", Value: m.cfg.ControlPlaneDSN},
		{Name: "NATS_URL", Value: m.cfg.NATSURL},
	}
	if m.cfg.NATSStream != "" {
		env = append(env, corev1.EnvVar{Name: "NATS_STREAM", Value: m.cfg.NATSStream})
	}
	if m.cfg.NATSSubjects != "" {
		env = append(env, corev1.EnvVar{Name: "NATS_SUBJECTS", Value: m.cfg.NATSSubjects})
	}
	for _, key := range m.cfg.PassthroughEnv {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, corev1.EnvVar{Name: key, Value: val})
		}
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
				"app.kubernetes.io/component": "ingestion-worker",
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
						"app.kubernetes.io/component": "ingestion-worker",
						"filament.galaxy.io/run-id":   string(spec.Run),
					},
				},
				Spec: podSpec,
			},
		},
	}
}
