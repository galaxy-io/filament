package k8s

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/galaxy-io/filament"
)

// workerResources converts the run's resolved quantities into the worker
// container's requirements. Each is optional: an unset quantity is left off the
// Job so a namespace LimitRange can supply it instead. Quantities are validated
// when written through the API, so a parse failure here means stored data went
// bad and the run is refused rather than silently sized wrong.
func workerResources(r filament.WorkerResources) (corev1.ResourceRequirements, error) {
	var out corev1.ResourceRequirements
	for _, q := range []struct {
		field string
		name  corev1.ResourceName
		value string
		into  *corev1.ResourceList
	}{
		{"cpu request", corev1.ResourceCPU, r.CPURequest, &out.Requests},
		{"memory request", corev1.ResourceMemory, r.MemoryRequest, &out.Requests},
		{"cpu limit", corev1.ResourceCPU, r.CPULimit, &out.Limits},
		{"memory limit", corev1.ResourceMemory, r.MemoryLimit, &out.Limits},
	} {
		if q.value == "" {
			continue
		}
		parsed, err := resource.ParseQuantity(q.value)
		if err != nil {
			return corev1.ResourceRequirements{}, fmt.Errorf("k8sdispatch: %s %q: %w", q.field, q.value, err)
		}
		if *q.into == nil {
			*q.into = corev1.ResourceList{}
		}
		(*q.into)[q.name] = parsed
	}
	return out, nil
}

// jobLabels is the label set for one dispatched run. The Job and its pod
// template share it: when the two were written out separately the pod template
// lost the tenant label, so a selector that found the Job missed its pods.
func (m *Module) jobLabels(spec filament.RunSpec) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      m.appName(),
		"app.kubernetes.io/component": "worker",
		"filament.galaxy.io/run-id":   string(spec.Run),
		"filament.galaxy.io/tenant":   string(spec.Tenant),
	}
}

// appName is the chart's app name, so Jobs select alongside chart-rendered
// resources under a nameOverride. It cannot be derived from anything else the
// dispatcher knows, so the chart passes it; unset means a plain install.
func (m *Module) appName() string {
	if m.cfg.WorkerAppName != "" {
		return m.cfg.WorkerAppName
	}
	return "filament"
}

func (m *Module) jobForSpec(spec filament.RunSpec) (*batchv1.Job, error) {
	name := jobName(m.cfg.JobNamePrefix, spec.Run)
	// All worker configuration arrives through the worker Secret and ConfigMap;
	// RUN_ID is the only value dispatch itself knows.
	envFrom := []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: m.cfg.WorkerSecretName}},
	}}
	if m.cfg.WorkerConfigMapName != "" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: m.cfg.WorkerConfigMapName}},
		})
	}
	env := []corev1.EnvVar{{Name: "RUN_ID", Value: string(spec.Run)}}

	resources, err := workerResources(spec.WorkerConfiguration.Resources)
	if err != nil {
		return nil, err
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
			Resources:       resources,
		}},
		TerminationGracePeriodSeconds: m.cfg.WorkerTerminationGraceSecs,
		ActiveDeadlineSeconds:         m.cfg.WorkerActiveDeadlineSeconds,
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: m.jobLabels(spec),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &m.cfg.BackoffLimit,
			TTLSecondsAfterFinished: m.cfg.TTLSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: m.jobLabels(spec)},
				Spec:       podSpec,
			},
		},
	}, nil
}
