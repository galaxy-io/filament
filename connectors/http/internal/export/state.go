package export

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const (
	scopedCheckpointVersion  = 1
	maxExportParents         = 10000
	maxExportCheckpointBytes = 16 << 20
)

// RunState keeps job state and committed watermarks in separate fields.
// A done job's candidate becomes the next run's watermark only after sink commit.
type RunState struct {
	Version     int                          `json:"version"`
	Identity    string                       `json:"identity"`
	Incremental bool                         `json:"incremental"`
	Initialized bool                         `json:"initialized"`
	Selected    []string                     `json:"selected,omitempty"`
	Parents     map[string]exportParentState `json:"parents"`
}

type exportParentState struct {
	Parent    map[string]string `json:"parent,omitempty"`
	Committed string            `json:"committed,omitempty"`
	Job       *exportScopedJob  `json:"job,omitempty"`
}

type exportScopedJob struct {
	Phase     string            `json:"phase"`
	Deadline  time.Time         `json:"deadline"`
	Seed      string            `json:"seed,omitempty"`
	Overlap   int               `json:"overlap,omitempty"`
	Captures  map[string]string `json:"captures,omitempty"`
	Candidate string            `json:"candidate,omitempty"`
}

// Scoped reports whether a resource needs per-parent or incremental job state.
func Scoped(res manifest.Resource) bool {
	return res.Export != nil && (res.Parent != nil || res.Incremental != nil)
}

func (c *Runtime) exportDefinitionIdentity(res manifest.Resource) string {
	config := c.IdentityConfig
	raw, _ := json.Marshal(struct {
		Resource manifest.Resource
		BaseURL  string
		Config   map[string]string
	}{res, c.BaseURL, config})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (c *Runtime) emptyExportRun(res manifest.Resource, inc bool) RunState {
	return RunState{Version: scopedCheckpointVersion, Identity: c.exportDefinitionIdentity(res), Incremental: inc, Parents: map[string]exportParentState{}}
}

func (s RunState) token() (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	if len(raw) > maxExportCheckpointBytes {
		return "", fmt.Errorf("export checkpoint exceeds 16 MiB; reduce the selected parent set")
	}
	return string(raw), nil
}

func (c *Runtime) decodeExportRun(res manifest.Resource, token string, inc bool) (RunState, error) {
	state := c.emptyExportRun(res, inc)
	if token == "" {
		return state, nil
	}
	if len(token) > maxExportCheckpointBytes {
		return state, fmt.Errorf("export checkpoint exceeds 16 MiB")
	}
	if !json.Valid([]byte(token)) {
		return state, fmt.Errorf("export %s: invalid checkpoint JSON", res.Name)
	}
	var saved RunState
	decoder := json.NewDecoder(strings.NewReader(token))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil || saved.Version != scopedCheckpointVersion || saved.Identity != state.Identity || saved.Incremental != inc || saved.Parents == nil {
		return state, fmt.Errorf("export %s: incompatible scoped checkpoint", res.Name)
	}
	if err := saved.validate(); err != nil {
		return state, fmt.Errorf("export %s checkpoint: %w", res.Name, err)
	}
	return saved, nil
}

func (s RunState) validate() error {
	if len(s.Parents) > maxExportParents || len(s.Selected) > maxExportParents {
		return fmt.Errorf("too many export parents")
	}
	if !s.Initialized && len(s.Selected) > 0 {
		return fmt.Errorf("uninitialized parent set")
	}
	seen := map[string]bool{}
	for _, key := range s.Selected {
		parent, ok := s.Parents[key]
		if !ok || parent.Job == nil || seen[key] {
			return fmt.Errorf("invalid selected parent")
		}
		seen[key] = true
		job := parent.Job
		switch job.Phase {
		case "planned":
		case "creating", "waiting", "done":
			if job.Deadline.IsZero() {
				return fmt.Errorf("missing job deadline")
			}
		default:
			return fmt.Errorf("invalid job phase")
		}
		if (job.Phase == "waiting" || job.Phase == "done") && len(job.Captures) == 0 {
			return fmt.Errorf("missing job captures")
		}
	}
	for key, parent := range s.Parents {
		if parent.Job != nil && !seen[key] {
			return fmt.Errorf("job outside selected parent set")
		}
	}
	return nil
}

// Checkpoint wraps job state in the engine's replayable checkpoint envelope.
func Checkpoint(res string, state RunState) (filament.Checkpoint, error) {
	token, err := state.token()
	if err != nil {
		return nil, err
	}
	mode := checkpoint.ModeKeyset
	if state.Incremental {
		mode = checkpoint.ModeIncremental
	}
	return checkpoint.KeysetCheckpoint{Mode: mode, Cols: []string{"export_state"}, Types: []string{"string"}, Meta: map[string]string{checkpoint.ReplayOnResume: "true"}, Shards: []checkpoint.KeysetShard{{Key: []string{token}}}}.ToCheckpoint(res), nil
}
