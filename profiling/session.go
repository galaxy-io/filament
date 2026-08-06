// Package profiling provides drop-in helpers for live and file-based Go
// profiling.
//
// Mount adds the standard /debug/pprof endpoints to an existing HTTP mux.
// Start records a bounded profiling session to disk. The default session
// records CPU activity while it is running and writes a heap snapshot when it
// stops:
//
//	session, err := profiling.Start(profiling.DefaultConfig("./profiles"))
//	if err != nil {
//		return err
//	}
//	defer session.Stop()
package profiling

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"sync"
	"time"
)

const (
	defaultBlockRate     = 1_000_000
	defaultMutexFraction = 5
)

// Profile identifies a profile captured by a Session.
type Profile string

const (
	// CPUProfile samples CPU activity for the lifetime of the session.
	CPUProfile Profile = "cpu"
	// TraceProfile records a runtime execution trace for the lifetime of the session.
	TraceProfile Profile = "trace"
	// HeapProfile records sampled in-use memory when the session stops.
	HeapProfile Profile = "heap"
	// AllocsProfile records sampled historical allocations when the session stops.
	AllocsProfile Profile = "allocs"
	// GoroutineProfile records all current goroutine stacks when the session stops.
	GoroutineProfile Profile = "goroutine"
	// BlockProfile records blocking events over the lifetime of the session.
	BlockProfile Profile = "block"
	// MutexProfile records contended mutex events over the lifetime of the session.
	MutexProfile Profile = "mutex"
	// ThreadCreateProfile records stacks that created OS threads when the session stops.
	ThreadCreateProfile Profile = "threadcreate"
)

var (
	// ErrActive is returned when another file-based profiling session is active.
	// The Go runtime supports only one process-wide CPU profile and trace at a time.
	ErrActive = errors.New("profiling: another session is active")

	activeMu sync.Mutex
	active   bool
)

// Config controls a file-based profiling session.
type Config struct {
	// Directory receives the profile files and is created when necessary.
	Directory string
	// Prefix is prepended to each filename. When empty, Start uses a UTC timestamp.
	Prefix string
	// Profiles selects what to capture. At least one profile is required.
	Profiles []Profile
	// BlockRate is the average nanoseconds between sampled blocking events.
	// Zero uses 1 millisecond. It applies only when BlockProfile is selected.
	// Stop disables the process-wide block profiler because the runtime does not
	// expose its previous rate.
	BlockRate int
	// MutexFraction samples one in this many contended mutex events.
	// Zero uses 5. It applies only when MutexProfile is selected.
	MutexFraction int
	// GCBeforeHeap runs garbage collection before writing HeapProfile.
	GCBeforeHeap bool
}

// DefaultConfig returns a CPU and heap profiling configuration. Stop the
// returned Session to flush both profiles.
func DefaultConfig(directory string) Config {
	return Config{
		Directory:    directory,
		Profiles:     []Profile{CPUProfile, HeapProfile},
		GCBeforeHeap: true,
	}
}

// Session is an active file-based profiling session. A process may have only
// one Session at a time. Stop is safe to call more than once.
type Session struct {
	cfg              Config
	files            map[Profile]string
	cpuFile          *os.File
	traceFile        *os.File
	cpuStarted       bool
	traceStarted     bool
	blockEnabled     bool
	mutexEnabled     bool
	oldMutexFraction int
	stopOnce         sync.Once
	stopErr          error
}

// Start begins a file-based profiling session. CPU, trace, block, and mutex
// profiles collect until Stop; the other selected profiles are snapshots taken
// by Stop.
func Start(cfg Config) (*Session, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	cfg.Profiles = slices.Clone(cfg.Profiles)
	if cfg.Prefix == "" {
		cfg.Prefix = time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	if err := os.MkdirAll(cfg.Directory, 0o750); err != nil {
		return nil, fmt.Errorf("profiling: create output directory: %w", err)
	}

	activeMu.Lock()
	if active {
		activeMu.Unlock()
		return nil, ErrActive
	}
	active = true
	activeMu.Unlock()

	s := &Session{cfg: cfg, files: make(map[Profile]string)}
	for _, profile := range cfg.Profiles {
		ext := ".pprof"
		if profile == TraceProfile {
			ext = ".out"
		}
		s.files[profile] = filepath.Join(cfg.Directory, cfg.Prefix+"-"+string(profile)+ext)
	}
	if err := s.startContinuousProfiles(); err != nil {
		s.abort()
		return nil, err
	}
	return s, nil
}

// Path returns the output path for profile and whether it was selected.
func (s *Session) Path(profile Profile) (string, bool) {
	path, ok := s.files[profile]
	return path, ok
}

// Files returns a copy of the selected profile-to-output-path mapping.
func (s *Session) Files() map[Profile]string {
	files := make(map[Profile]string, len(s.files))
	for profile, path := range s.files {
		files[profile] = path
	}
	return files
}

// Stop ends continuous profiles, writes selected snapshots, and closes all
// output files. Its result is cached, so repeated calls return the same error.
func (s *Session) Stop() error {
	s.stopOnce.Do(func() {
		var errs []error
		if s.traceFile != nil {
			trace.Stop()
			errs = append(errs, closeProfileFile(TraceProfile, s.traceFile))
		}
		if s.cpuFile != nil {
			pprof.StopCPUProfile()
			errs = append(errs, closeProfileFile(CPUProfile, s.cpuFile))
		}
		if s.cfg.GCBeforeHeap && slices.Contains(s.cfg.Profiles, HeapProfile) {
			runtime.GC()
		}
		for _, profile := range s.cfg.Profiles {
			if profile != CPUProfile && profile != TraceProfile {
				errs = append(errs, s.writeSnapshot(profile))
			}
		}
		if s.blockEnabled {
			runtime.SetBlockProfileRate(0)
		}
		if s.mutexEnabled {
			runtime.SetMutexProfileFraction(s.oldMutexFraction)
		}
		s.stopErr = errors.Join(errs...)
		setInactive()
	})
	return s.stopErr
}

func validateConfig(cfg Config) error {
	if cfg.Directory == "" {
		return errors.New("profiling: output directory is required")
	}
	if len(cfg.Profiles) == 0 {
		return errors.New("profiling: at least one profile is required")
	}
	if cfg.Prefix != "" && (filepath.Base(cfg.Prefix) != cfg.Prefix || cfg.Prefix == ".") {
		return errors.New("profiling: prefix must be a filename, not a path")
	}
	if cfg.BlockRate < 0 {
		return errors.New("profiling: block rate cannot be negative")
	}
	if cfg.MutexFraction < 0 {
		return errors.New("profiling: mutex fraction cannot be negative")
	}
	seen := make(map[Profile]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if !validProfile(profile) {
			return fmt.Errorf("profiling: unknown profile %q", profile)
		}
		if _, ok := seen[profile]; ok {
			return fmt.Errorf("profiling: duplicate profile %q", profile)
		}
		seen[profile] = struct{}{}
	}
	return nil
}

func validProfile(profile Profile) bool {
	switch profile {
	case CPUProfile, TraceProfile, HeapProfile, AllocsProfile, GoroutineProfile,
		BlockProfile, MutexProfile, ThreadCreateProfile:
		return true
	default:
		return false
	}
}

func (s *Session) startContinuousProfiles() error {
	if slices.Contains(s.cfg.Profiles, CPUProfile) {
		f, err := createProfileFile(s.files[CPUProfile])
		if err != nil {
			return err
		}
		s.cpuFile = f
		if err := pprof.StartCPUProfile(f); err != nil {
			return fmt.Errorf("profiling: start CPU profile: %w", err)
		}
		s.cpuStarted = true
	}
	if slices.Contains(s.cfg.Profiles, TraceProfile) {
		f, err := createProfileFile(s.files[TraceProfile])
		if err != nil {
			return err
		}
		s.traceFile = f
		if err := trace.Start(f); err != nil {
			return fmt.Errorf("profiling: start trace: %w", err)
		}
		s.traceStarted = true
	}
	if slices.Contains(s.cfg.Profiles, BlockProfile) {
		rate := s.cfg.BlockRate
		if rate == 0 {
			rate = defaultBlockRate
		}
		runtime.SetBlockProfileRate(rate)
		s.blockEnabled = true
	}
	if slices.Contains(s.cfg.Profiles, MutexProfile) {
		fraction := s.cfg.MutexFraction
		if fraction == 0 {
			fraction = defaultMutexFraction
		}
		s.oldMutexFraction = runtime.SetMutexProfileFraction(fraction)
		s.mutexEnabled = true
	}
	return nil
}

func (s *Session) writeSnapshot(profile Profile) error {
	p := pprof.Lookup(string(profile))
	if p == nil {
		return fmt.Errorf("profiling: runtime profile %q is unavailable", profile)
	}
	f, err := createProfileFile(s.files[profile])
	if err != nil {
		return err
	}
	writeErr := p.WriteTo(f, 0)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("profiling: write %s profile: %w", profile, err)
	}
	return nil
}

func createProfileFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("profiling: create %q: %w", path, err)
	}
	return f, nil
}

func closeProfileFile(profile Profile, f *os.File) error {
	if err := f.Close(); err != nil {
		return fmt.Errorf("profiling: close %s profile: %w", profile, err)
	}
	return nil
}

func (s *Session) abort() {
	if s.traceStarted {
		trace.Stop()
	}
	if s.traceFile != nil {
		_ = s.traceFile.Close()
		_ = os.Remove(s.traceFile.Name())
	}
	if s.cpuStarted {
		pprof.StopCPUProfile()
	}
	if s.cpuFile != nil {
		_ = s.cpuFile.Close()
		_ = os.Remove(s.cpuFile.Name())
	}
	if s.blockEnabled {
		runtime.SetBlockProfileRate(0)
	}
	if s.mutexEnabled {
		runtime.SetMutexProfileFraction(s.oldMutexFraction)
	}
	setInactive()
}

func setInactive() {
	activeMu.Lock()
	active = false
	activeMu.Unlock()
}
