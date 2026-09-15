package dado

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/atterpac/dado/inline"
)

// RunLoading renders Filament's inline activity spinner while fn runs. Calls
// to setLabel update the live activity text. Non-terminal callers run fn
// without rendering.
func (r *Renderer) RunLoading(label string, fn func(setLabel func(string)) error) (runErr error) {
	if !r.interactiveAvailable() {
		return fn(func(string) {})
	}
	renderer := inline.NewRenderer(inline.WithOutput(r.statusWriter()))
	r.interactiveRenderer = renderer
	defer func() {
		runErr = errors.Join(runErr, renderer.Clear(), renderer.Close())
		r.interactiveRenderer = nil
	}()
	return r.loadingWithUpdates(label, fn)
}

// loading keeps a one-line spinner in the live frame while fn runs. On
// success the row is left for the next screen's first frame to overwrite in
// one write, so nothing blank flashes between them; on failure it is cleared.
// Without a live renderer fn simply runs.
func (r *Renderer) loading(label string, fn func() error) error {
	return r.loadingWithUpdates(label, func(func(string)) error { return fn() })
}

func (r *Renderer) loadingWithUpdates(label string, fn func(setLabel func(string)) error) error {
	if r.interactiveRenderer == nil {
		return fn(func(string) {})
	}
	var current atomic.Value
	current.Store(label)
	done := make(chan error, 1)
	go func() { done <- fn(func(label string) { current.Store(label) }) }()
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	width := interactiveRenderWidth(r.statusWriter())
	frames := r.theme.Status.SpinnerFrames
	if len(frames) == 0 {
		frames = []string{"·"}
	}
	for tick := 0; ; tick++ {
		frame := inline.NewFrame(width, 1)
		x := draw(frame, 0, 0, frames[tick%len(frames)], r.theme.Accent)
		draw(frame, x+1, 0, current.Load().(string), r.theme.Muted)
		if err := r.interactiveRenderer.Render(frame); err != nil {
			return errors.Join(err, <-done)
		}
		select {
		case err := <-done:
			if err != nil {
				return errors.Join(err, r.interactiveRenderer.Clear())
			}
			return nil
		case <-ticker.C:
		}
	}
}
