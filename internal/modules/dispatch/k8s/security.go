package k8s

import corev1 "k8s.io/api/core/v1"

// Worker pods carry the restricted Pod Security Standard so a namespace that
// enforces it admits them. The image is distroless nonroot and the binary
// writes nothing to disk, so nothing here needs loosening per run.

func workerPodSecurity() *corev1.PodSecurityContext {
	runAsNonRoot := true
	nonroot := int64(65532)
	return &corev1.PodSecurityContext{
		RunAsNonRoot:   &runAsNonRoot,
		RunAsUser:      &nonroot,
		RunAsGroup:     &nonroot,
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

func workerContainerSecurity() *corev1.SecurityContext {
	noPrivilegeEscalation := false
	readOnlyRoot := true
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &noPrivilegeEscalation,
		ReadOnlyRootFilesystem:   &readOnlyRoot,
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}
