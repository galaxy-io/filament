package k8sdispatch

import (
	"os"

	ingestion "github.com/galaxy-io/filament"
)

func (m *Module) jobForSpec(spec ingestion.RunSpec) job {
	name := jobName(m.cfg.JobNamePrefix, spec.Run)
	env := []envVar{
		{Name: "RUN_ID", Value: string(spec.Run)},
		{Name: "CONTROL_PLANE_DSN", Value: m.cfg.ControlPlaneDSN},
		{Name: "NATS_URL", Value: m.cfg.NATSURL},
	}
	if m.cfg.NATSStream != "" {
		env = append(env, envVar{Name: "NATS_STREAM", Value: m.cfg.NATSStream})
	}
	if m.cfg.NATSSubjects != "" {
		env = append(env, envVar{Name: "NATS_SUBJECTS", Value: m.cfg.NATSSubjects})
	}
	for _, key := range m.cfg.PassthroughEnv {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, envVar{Name: key, Value: val})
		}
	}

	podSpec := podSpec{
		RestartPolicy: m.cfg.WorkerRestartPolicy,
		Containers: []container{{
			Name:            "worker",
			Image:           m.cfg.WorkerImage,
			ImagePullPolicy: m.cfg.WorkerImagePullPolicy,
			Env:             env,
		}},
		ServiceAccountName:            m.cfg.WorkerServiceAccount,
		TerminationGracePeriodSeconds: m.cfg.WorkerTerminationGraceSecs,
		ActiveDeadlineSeconds:         m.cfg.WorkerActiveDeadlineSeconds,
	}
	return job{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Metadata: metadata{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "filament",
				"app.kubernetes.io/component": "ingestion-worker",
				"filament.galaxy.io/run-id":   string(spec.Run),
				"filament.galaxy.io/tenant":   string(spec.Tenant),
			},
		},
		Spec: jobSpec{
			BackoffLimit:            &m.cfg.BackoffLimit,
			TTLSecondsAfterFinished: m.cfg.TTLSecondsAfterFinished,
			Template: podTemplate{
				Metadata: metadata{Labels: map[string]string{
					"app.kubernetes.io/name":      "filament",
					"app.kubernetes.io/component": "ingestion-worker",
					"filament.galaxy.io/run-id":   string(spec.Run),
				}},
				Spec: podSpec,
			},
		},
	}
}

type job struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   metadata `json:"metadata"`
	Spec       jobSpec  `json:"spec"`
}

type metadata struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type jobSpec struct {
	BackoffLimit            *int32      `json:"backoffLimit,omitempty"`
	TTLSecondsAfterFinished *int32      `json:"ttlSecondsAfterFinished,omitempty"`
	Template                podTemplate `json:"template"`
}

type podTemplate struct {
	Metadata metadata `json:"metadata,omitempty"`
	Spec     podSpec  `json:"spec"`
}

type podSpec struct {
	RestartPolicy                 string      `json:"restartPolicy"`
	ServiceAccountName            string      `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64      `json:"terminationGracePeriodSeconds,omitempty"`
	ActiveDeadlineSeconds         *int64      `json:"activeDeadlineSeconds,omitempty"`
	Containers                    []container `json:"containers"`
}

type container struct {
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	ImagePullPolicy string   `json:"imagePullPolicy,omitempty"`
	Env             []envVar `json:"env,omitempty"`
}

type envVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
