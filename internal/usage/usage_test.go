package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCgroup(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cpu.stat", "usage_usec 1500000\nuser_usec 1000000\nsystem_usec 500000\n")
	write("memory.current", "104857600\n")
	write("memory.peak", "209715200\n")

	s, ok := read(dir)
	if !ok {
		t.Fatal("expected ok")
	}
	if s.CPUSeconds != 1.5 {
		t.Errorf("CPUSeconds = %v, want 1.5", s.CPUSeconds)
	}
	if s.MemoryBytes != 104857600 {
		t.Errorf("MemoryBytes = %d", s.MemoryBytes)
	}
	if s.MemoryPeakBytes != 209715200 {
		t.Errorf("MemoryPeakBytes = %d", s.MemoryPeakBytes)
	}
}

func TestReadNoPeak(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cpu.stat"), []byte("usage_usec 250000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memory.current"), []byte("1024"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, ok := read(dir)
	if !ok {
		t.Fatal("expected ok")
	}
	if s.CPUSeconds != 0.25 || s.MemoryBytes != 1024 || s.MemoryPeakBytes != 0 {
		t.Errorf("unexpected sample %+v", s)
	}
}

func TestReadMissing(t *testing.T) {
	if _, ok := read(t.TempDir()); ok {
		t.Fatal("expected ok=false with no cgroup files")
	}
}
