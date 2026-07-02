package k8sdispatch

import (
	"os"
	"strconv"
	"strings"

	ingestion "github.com/galaxy-io/filament"
)

func jobName(prefix string, run ingestion.RunID) string {
	p := cleanDNS1123(prefix)
	r := cleanDNS1123(string(run))
	name := p + "-" + r
	if len(name) <= 63 {
		return name
	}
	if len(r) > 48 {
		r = r[:48]
	}
	name = p + "-" + r
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.Trim(name, "-")
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

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func boolEnv(key string) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
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
