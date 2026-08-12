// Package usage samples this process's cgroup v2 CPU and memory accounting.
// In a worker pod that is the pod's own usage as Kubernetes accounts it; the
// sink for these numbers is the run.heartbeat fact and the worker's OTel
// instruments.
package usage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultRoot = "/sys/fs/cgroup"

// Sample is one point-in-time read of the cgroup's resource accounting.
type Sample struct {
	CPUSeconds      float64 // cumulative CPU time consumed
	MemoryBytes     int64   // current working set
	MemoryPeakBytes int64   // high-water working set (0 when the kernel doesn't expose it)
}

// Read returns the current cgroup v2 usage. ok is false when no cgroup
// filesystem is available (a dev run outside a pod), in which case callers
// skip the beat rather than report zeros.
func Read() (Sample, bool) {
	return read(defaultRoot)
}

func read(root string) (Sample, bool) {
	var s Sample
	cpu, cpuOK := cpuSeconds(filepath.Join(root, "cpu.stat"))
	mem, memOK := readInt(filepath.Join(root, "memory.current"))
	if !cpuOK && !memOK {
		return Sample{}, false
	}
	s.CPUSeconds = cpu
	s.MemoryBytes = mem
	if peak, ok := readInt(filepath.Join(root, "memory.peak")); ok {
		s.MemoryPeakBytes = peak
	}
	return s, true
}

// cpuSeconds parses usage_usec out of cpu.stat.
func cpuSeconds(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			usec, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, false
			}
			return float64(usec) / 1e6, true
		}
	}
	return 0, false
}

// readInt reads a file holding a single integer, e.g. memory.current.
func readInt(path string) (int64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
