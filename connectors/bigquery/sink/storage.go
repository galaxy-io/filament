package bigquery

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery/storage/apiv1/storagepb"
	"github.com/galaxy-io/filament"
)

func (s *Sink) prepareUpload(ctx context.Context, state *tableState) error {
	started := time.Now()
	defer s.logTiming("prepare", state.definition.name, started)
	slog.InfoContext(ctx, "Preparing BigQuery upload", "component", "bigquery", "event.name", "bigquery.write.prepare.started", "run_id", s.run, "resource", state.definition.name)
	phaseStarted := time.Now()
	err := s.ensureDestination(ctx, state)
	s.logTiming("destination_metadata", state.definition.name, phaseStarted)
	if err != nil {
		return err
	}
	phaseStarted = time.Now()
	err = s.createStage(ctx, state)
	s.logTiming("stage_metadata", state.definition.name, phaseStarted)
	if err != nil {
		return err
	}
	phaseStarted = time.Now()
	stream, err := s.storage.CreateWriteStream(ctx, &storagepb.CreateWriteStreamRequest{Parent: s.tablePath(state.stage), WriteStream: &storagepb.WriteStream{Type: storagepb.WriteStream_PENDING}})
	if err != nil {
		return err
	}
	s.logTiming("create_stream", state.definition.name, phaseStarted)
	state.stream = stream.GetName()
	return nil
}
func (s *Sink) tablePath(table string) string {
	return fmt.Sprintf("projects/%s/datasets/%s/tables/%s", s.project, s.dataset, table)
}

func (s *Sink) publish(ctx context.Context, state *tableState) error {
	started := time.Now()
	defer s.logTiming("publish", state.definition.name, started, "records", state.rows)
	slog.InfoContext(ctx, "Publishing BigQuery resource", "component", "bigquery", "event.name", "bigquery.write.publish.started", "run_id", s.run, "resource", state.definition.name, "records", state.rows)
	mode := s.modeFor(state.definition.name)
	if state.stream == "" {
		if err := s.ensureDestination(ctx, state); err != nil {
			return err
		}
		if mode != filament.WriteReplace {
			return nil
		}
	} else {
		final, err := s.storage.FinalizeWriteStream(ctx, &storagepb.FinalizeWriteStreamRequest{Name: state.stream})
		if err != nil {
			return err
		}
		if final.RowCount != state.rows {
			return fmt.Errorf("stream finalized %d rows, expected %d", final.RowCount, state.rows)
		}
		committed, err := s.storage.BatchCommitWriteStreams(ctx, &storagepb.BatchCommitWriteStreamsRequest{Parent: s.tablePath(state.stage), WriteStreams: []string{state.stream}})
		if err != nil {
			return err
		}
		if len(committed.StreamErrors) > 0 {
			return fmt.Errorf("commit stream: %v", committed.StreamErrors)
		}
		if committed.CommitTime == nil {
			return fmt.Errorf("stream commit returned no commit time")
		}
	}
	table := state.definition
	stage := qualified(s.project, s.dataset, state.stage)
	var statement string
	switch mode {
	case filament.WriteReplace:
		if state.rows == 0 {
			statement = "TRUNCATE TABLE " + table.qualified
		} else {
			statement = replaceTableSQL(table, stage)
		}
	case filament.WriteAppend:
		statement = insertTableSQL(table, table.qualified, stage)
	case filament.WriteUpsert, filament.WriteMerge, filament.WriteDelete:
		statement = mergeTableSQL(table, stage)
	default:
		return fmt.Errorf("unsupported publication mode %q", mode)
	}
	state.publishing = true
	digest := sha256.New().Sum(nil)
	if state.segment != nil {
		digest = state.segment.Sum(nil)
	}
	statement = fmt.Sprintf("-- filament-segment:%x\n%s", digest, statement)
	// A resume has a new durable dispatch identity; redelivery retains it.
	// Missing execution IDs intentionally fail closed on differing replays.
	_, err := s.runQuery(ctx, statement, jobID(s.run, table.qualified, "publish_v3", 0, 0, s.execution+"\x00"+string(mode)))
	if err == nil {
		state.published = true
	}
	return err
}
