package iceberg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	iceberg "github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
)

// streamRotateAfter bounds how long one data file stream runs. A stream holds
// the storage credentials of the table it was loaded with, and catalogs that
// vend credentials expire them, so a long run restarts its stream from a
// freshly loaded table.
var streamRotateAfter = 15 * time.Minute

// streamQueue is the number of batches queued ahead of the Parquet writer.
const streamQueue = 4

// appendFiles writes one resource's append batches to Parquet data files while
// the run is in progress, so Commit only attaches finished files instead of
// writing the whole run. Files stay invisible until Commit adds them to the
// table, and an aborted run deletes them.
type appendFiles struct {
	load      func(context.Context) (*icetable.Table, error)
	current   *dataFileStream
	files     []iceberg.DataFile
	tbl       *icetable.Table // most recent table, for its file IO
	attempted bool            // a commit was attempted; files must not be deleted
}

func newAppendFiles(cat icecatalog.Catalog, ident icetable.Identifier) *appendFiles {
	return &appendFiles{load: func(ctx context.Context) (*icetable.Table, error) {
		return cat.LoadTable(ctx, ident)
	}}
}

// write conforms rows to the table schema and queues them for the writer.
func (a *appendFiles) write(ctx context.Context, rows arrow.RecordBatch) error {
	if a.current != nil && time.Since(a.current.started) >= streamRotateAfter {
		if err := a.finishCurrent(); err != nil {
			return err
		}
	}
	if a.current == nil {
		tbl, err := a.load(ctx)
		if err != nil {
			return fmt.Errorf("load table: %w", err)
		}
		stream, err := startDataFileStream(tbl)
		if err != nil {
			return err
		}
		a.tbl, a.current = tbl, stream
	}
	rec, err := conform(rows, a.current.schema)
	if err != nil {
		return err
	}
	return a.current.write(ctx, rec)
}

func (a *appendFiles) finishCurrent() error {
	if a.current == nil {
		return nil
	}
	files, err := a.current.finish()
	a.files = append(a.files, files...)
	a.current = nil
	return err
}

// commit finishes writing and adds every file to the table in one snapshot.
func (a *appendFiles) commit(ctx context.Context, tbl *icetable.Table) (*icetable.Table, error) {
	if err := a.finishCurrent(); err != nil {
		return nil, fmt.Errorf("write data files: %w", err)
	}
	if len(a.files) == 0 {
		return tbl, nil
	}
	txn := tbl.NewTransaction()
	if err := txn.AddDataFiles(ctx, a.files, nil); err != nil {
		return nil, fmt.Errorf("add data files: %w", err)
	}
	a.attempted = true
	return txn.Commit(ctx)
}

// abort stops the writer and deletes the files it wrote, unless a commit was
// attempted: a failed commit may still have landed, and its files belong to
// the table.
func (a *appendFiles) abort(ctx context.Context) error {
	if a.current != nil {
		a.files = append(a.files, a.current.abort()...)
		a.current = nil
	}
	if a.attempted || len(a.files) == 0 || a.tbl == nil {
		return nil
	}
	fs, err := a.tbl.FS(ctx)
	if err != nil {
		return fmt.Errorf("remove uncommitted data files: %w", err)
	}
	var errs []error
	for _, file := range a.files {
		if err := fs.Remove(file.FilePath()); err != nil {
			errs = append(errs, fmt.Errorf("remove uncommitted data file %s: %w", file.FilePath(), err))
		}
	}
	a.files = nil
	return errors.Join(errs...)
}

// dataFileStream feeds batches to one iceberg-go WriteRecords call running in
// the background and collects the data files it produces.
type dataFileStream struct {
	schema  *arrow.Schema
	started time.Time
	cancel  context.CancelFunc
	records chan arrow.RecordBatch
	closed  bool
	done    chan struct{}
	files   []iceberg.DataFile // valid after done
	err     error              // valid after done
}

func startDataFileStream(tbl *icetable.Table) (*dataFileStream, error) {
	schema, err := icetable.SchemaToArrowSchema(tbl.Schema(), nil, false, false)
	if err != nil {
		return nil, fmt.Errorf("build arrow schema: %w", err)
	}
	// The stream outlives any one Apply call; Commit or Abort ends it.
	ctx, cancel := context.WithCancel(context.Background())
	s := &dataFileStream{
		schema: schema, started: time.Now(), cancel: cancel,
		records: make(chan arrow.RecordBatch, streamQueue), done: make(chan struct{}),
	}
	go s.run(ctx, tbl)
	return s, nil
}

func (s *dataFileStream) run(ctx context.Context, tbl *icetable.Table) {
	defer close(s.done)
	records := func(yield func(arrow.RecordBatch, error) bool) {
		for rec := range s.records {
			if !yield(rec, nil) {
				return
			}
		}
	}
	for file, err := range icetable.WriteRecords(ctx, tbl, s.schema, records) {
		if err != nil {
			s.err = err
			return
		}
		s.files = append(s.files, file)
	}
}

// write hands rec to the writer, which releases it.
func (s *dataFileStream) write(ctx context.Context, rec arrow.RecordBatch) error {
	select {
	case s.records <- rec:
		return nil
	case <-s.done:
		rec.Release()
		if s.err != nil {
			return fmt.Errorf("write data files: %w", s.err)
		}
		return errors.New("write data files: writer stopped")
	case <-ctx.Done():
		rec.Release()
		return ctx.Err()
	}
}

func (s *dataFileStream) closeInput() {
	if !s.closed {
		s.closed = true
		close(s.records)
	}
}

// finish waits for queued batches to be written and returns the files.
func (s *dataFileStream) finish() ([]iceberg.DataFile, error) {
	s.closeInput()
	<-s.done
	s.cancel()
	s.drain()
	return s.files, s.err
}

// abort cancels the writer, which abandons its in-progress file, and returns
// the files it had already completed.
func (s *dataFileStream) abort() []iceberg.DataFile {
	s.cancel()
	s.closeInput()
	<-s.done
	s.drain()
	return s.files
}

// drain releases batches the writer never took after stopping early.
func (s *dataFileStream) drain() {
	for rec := range s.records {
		rec.Release()
	}
}
