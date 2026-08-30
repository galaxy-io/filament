package k8s

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/galaxy-io/filament"
)

func jobName(prefix string, run filament.RunID, executionID string) string {
	p := cleanDNS1123(prefix)
	r := cleanDNS1123(string(run))
	suffix := ""
	if executionID != "" {
		suffix = "-" + executionToken(executionID)
	}
	name := p + "-" + r + suffix
	if len(name) <= 63 {
		return name
	}
	maxRun := 63 - len(p) - len(suffix) - 1
	if maxRun < 1 {
		maxRun = 1
	}
	if len(r) > maxRun {
		r = r[:maxRun]
	}
	name = p + "-" + r + suffix
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.Trim(name, "-")
}

func executionToken(executionID string) string {
	sum := sha256.Sum256([]byte(executionID))
	return hex.EncodeToString(sum[:6])
}

// jobFinished reports whether the Job controller has stamped a terminal
// condition. Pod counters can read zero mid-reconcile, so this is the boundary
// the reaper's Alive probe trusts.
func jobFinished(j *batchv1.Job) bool {
	for _, condition := range j.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		if condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed {
			return true
		}
	}
	return false
}

// k8sLabelValue returns value when it is a valid label value, else a stable
// token derived from it.
func k8sLabelValue(value string) string {
	if len(validation.IsValidLabelValue(value)) == 0 {
		return value
	}
	return executionToken(value)
}

func cleanDNS1123(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "run"
	}
	return out
}

func isDNS1123Label(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		if r == '-' && i > 0 && i < len(s)-1 {
			continue
		}
		return false
	}
	return true
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func int32Env(key string, fallback int32) int32 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(n)
}

func int64Env(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func readFileTrim(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
