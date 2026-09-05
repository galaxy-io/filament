package dado

import (
	"errors"
	"time"

	"github.com/atterpac/dado/inline"
)

// loading keeps a one-line spinner in the live frame while fn runs. On
// success the row is left for the next screen's first frame to overwrite in
// one write, so nothing blank flashes between them; on failure it is cleared.
// Without a live renderer fn simply runs.
func (r *Renderer) loading(label string, fn func() error) error {
	if r.interactiveRenderer == nil {
		return fn()
	}
	done := make(chan error, 1)
	go func() { done <- fn() }()
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
		draw(frame, x+1, 0, label, r.theme.Muted)
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
