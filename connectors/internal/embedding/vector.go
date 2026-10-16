package embedding

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// ParseVector decodes an embedding stored as a JSON array of numbers, such as
// "[0.1, 0.2, 0.3]". Filament carries array columns as text, so this is the
// form a precomputed vector arrives in. It rejects empty vectors, components
// that are not numbers or are null, and values that are not finite float32s,
// rather than writing a vector that does not match its source.
func ParseVector(s string) ([]float32, error) {
	var raw []*float64
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("vector is not a JSON array of numbers: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("vector is empty")
	}
	out := make([]float32, len(raw))
	for i, v := range raw {
		if v == nil {
			return nil, fmt.Errorf("vector component %d is null", i)
		}
		f := float32(*v)
		if math.IsNaN(*v) || math.IsInf(float64(f), 0) {
			return nil, fmt.Errorf("vector component %d (%g) is not a finite float32", i, *v)
		}
		out[i] = f
	}
	return out, nil
}
