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
// container's requirements. Each map is optional: an unset one is left off the
// Job so a namespace LimitRange can supply it instead. Quantities are validated
// when written through the API, so a parse failure here means stored data went
// bad and the run is refused rather than silently sized wrong.
func workerResources(r filament.WorkerResources) (corev1.ResourceRequirements, error) {
	var out corev1.ResourceRequirements
	var err error
	if out.Requests, err = resourceList("request", r.Requests); err != nil {
		return corev1.ResourceRequirements{}, err
	}
	if out.Limits, err = resourceList("limit", r.Limits); err != nil {
		return corev1.ResourceRequirements{}, err
	}
	return out, nil
}

func resourceList(kind string, in map[string]string) (corev1.ResourceList, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(corev1.ResourceList, len(in))
	for name, value := range in {
		parsed, err := resource.ParseQuantity(value)
		if err != nil {
			return nil, fmt.Errorf("k8sdispatch: %s %s %q: %w", name, kind, value, err)
		}
		out[corev1.ResourceName(name)] = parsed
	}
	return out, nil
}

// workerTolerations converts the run's tolerations to their Kubernetes form.
// Nil in, nil out, so an unplaced run leaves the field off the Job.
func workerTolerations(in []filament.WorkerToleration) []corev1.Toleration {
	if len(in) == 0 {
		return nil
	}
	out := make([]corev1.Toleration, 0, len(in))
	for _, t := range in {
		out = append(out, corev1.Toleration{
			Key:      t.Key,
			Operator: corev1.TolerationOperator(t.Operator),
			Value:    t.Value,
			Effect:   corev1.TaintEffect(t.Effect),
		})
	}
	return out
}

// jobLabels is the label set for one dispatched run. The Job and its pod
// template share it: when the two were written out separately the pod template
// lost the tenant label, so a selector that found the Job missed its pods.
func (m *Module) jobLabels(spec filament.RunSpec) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":      m.appName(),
		"app.kubernetes.io/component": "worker",
		"filament.galaxy.io/run-id":   string(spec.Run),
		"filament.galaxy.io/tenant":   string(spec.Tenant),
	}
	if spec.ExecutionID != "" {
		labels["filament.galaxy.io/execution-id"] = executionToken(spec.ExecutionID)
	}
	if spec.SourceConnectionID != "" {
		labels["filament.galaxy.io/source-connection-id"] = k8sLabelValue(spec.SourceConnectionID)
	}
	if spec.SinkConnectionID != "" {
		labels["filament.galaxy.io/sink-connection-id"] = k8sLabelValue(spec.SinkConnectionID)
	}
	return labels
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
	name := jobName(m.cfg.JobNamePrefix, spec.Run, spec.ExecutionID)
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
		NodeSelector:       spec.WorkerConfiguration.NodeSelector,
		Tolerations:        workerTolerations(spec.WorkerConfiguration.Tolerations),
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
