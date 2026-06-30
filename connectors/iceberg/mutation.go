package iceberg

import (
	"encoding/json"

	"github.com/galaxy-io/filament"
)

type mutationState struct {
	filterKeys []keyTuple
	live       []json.RawMessage
}

func collectMutationState(it *iceTable, rb *recordBuf, keys []string) (mutationState, error) {
	latest := map[string]recordEntry{}
	tuples := map[string]keyTuple{}
	order := []string{}
	err := rb.streamEntries(commitRowsPerChunk, func(entries []recordEntry) error {
		for _, entry := range entries {
			tuple, err := extractKeyTuple(entry.Data, it, keys)
			if err != nil {
				return err
			}
			encoded := tuple.key()
			if _, seen := tuples[encoded]; !seen {
				order = append(order, encoded)
				tuples[encoded] = tuple
			}
			latest[encoded] = entry
		}
		return nil
	})
	if err != nil {
		return mutationState{}, err
	}

	state := mutationState{filterKeys: make([]keyTuple, 0, len(order))}
	for _, encoded := range order {
		state.filterKeys = append(state.filterKeys, tuples[encoded])
		entry := latest[encoded]
		if entry.Op != ingestion.OpDelete {
			cp := make(json.RawMessage, len(entry.Data))
			copy(cp, entry.Data)
			state.live = append(state.live, cp)
		}
	}
	return state, nil
}

func recordsFromRaw(records []json.RawMessage) *recordBuf {
	rb := newRecordBuf(0)
	rb.mem = make([]recordEntry, len(records))
	for i := range records {
		rb.mem[i] = recordEntry{Op: ingestion.OpInsert, Data: records[i]}
	}
	return rb
}
