package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

// pump is the sole owner of PgConn. A bounded queue absorbs WAL while the
// pipeline writes. When the queue fills, status updates and certified acknowledgements
// still run, without allowing the decoder or sink to advance confirmed_flush_lsn.
func (s *walSession) pump(ctx context.Context) {
	defer close(s.done)
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if s.pumpErr == context.Canceled {
			s.pumpErr = nil
		}
		s.pumpErr = errors.Join(s.pumpErr, s.conn.Close(c))
	}()
	tick := time.NewTicker(standbyStatusInterval)
	defer tick.Stop()
	status := func(c context.Context, lsn pglogrepl.LSN) error {
		return pglogrepl.SendStandbyStatusUpdate(c, s.conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: lsn, WALFlushPosition: lsn, WALApplyPosition: lsn})
	}
	acknowledge := func(req walAck) error {
		err := ctx.Err()
		if err == nil {
			err = s.checkAuthority(req.ctx)
		}
		if err == nil && req.target < s.acknowledged {
			err = errors.New("postgres continuous: acknowledgement regressed")
		}
		if err == nil {
			err = status(req.ctx, req.target)
		}
		if err == nil {
			s.acknowledged = req.target
		}
		req.done <- err
		return err
	}
	if err := status(ctx, s.acknowledged); err != nil {
		s.pumpErr = err
		return
	}
	var pending *walEvent
	for {
		if pending != nil {
			select {
			case <-ctx.Done():
				s.pumpErr = ctx.Err()
				return
			case req := <-s.acks:
				if err := acknowledge(req); err != nil {
					s.pumpErr = err
					return
				}
			case <-tick.C:
				if err := status(ctx, s.acknowledged); err != nil {
					s.pumpErr = err
					return
				}
			case s.events <- *pending:
				pending = nil
			}
			continue
		}
		select {
		case <-ctx.Done():
			s.pumpErr = ctx.Err()
			return
		case req := <-s.acks:
			if err := acknowledge(req); err != nil {
				s.pumpErr = err
				return
			}
			continue
		case <-tick.C:
			if err := status(ctx, s.acknowledged); err != nil {
				s.pumpErr = err
				return
			}
			continue
		default:
		}
		// A short receive deadline gives queued acknowledgements bounded service
		// latency without concurrently using PgConn from another goroutine.
		receive, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		raw, err := s.conn.ReceiveMessage(receive)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				s.pumpErr = ctx.Err()
				return
			}
			if pgconn.Timeout(err) {
				continue
			}
			s.pumpErr = fmt.Errorf("postgres continuous: receive WAL: %w", err)
			return
		}
		pending, err = s.receiveWAL(ctx, raw)
		if err != nil {
			s.pumpErr = err
			return
		}
	}
}

func (s *walSession) receiveWAL(ctx context.Context, raw pgproto3.BackendMessage) (*walEvent, error) {
	switch m := raw.(type) {
	case *pgproto3.ErrorResponse:
		return nil, pgconn.ErrorResponseToPgError(m)
	case *pgproto3.CopyData:
		if len(m.Data) == 0 {
			return nil, nil
		}
		switch m.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			keepalive, err := pglogrepl.ParsePrimaryKeepaliveMessage(m.Data[1:])
			if err != nil {
				return nil, err
			}
			if keepalive.ReplyRequested {
				if err := pglogrepl.SendStandbyStatusUpdate(ctx, s.conn, pglogrepl.StandbyStatusUpdate{WALWritePosition: s.acknowledged, WALFlushPosition: s.acknowledged, WALApplyPosition: s.acknowledged}); err != nil {
					return nil, err
				}
			}
			return &walEvent{progress: keepalive.ServerWALEnd}, nil
		case pglogrepl.XLogDataByteID:
			data, err := pglogrepl.ParseXLogData(m.Data[1:])
			if err != nil {
				return nil, err
			}
			return &walEvent{data: append([]byte(nil), data.WALData...), start: data.WALStart}, nil
		default:
			return nil, fmt.Errorf("postgres continuous: unexpected replication message %q", m.Data[0])
		}
	default:
		return nil, fmt.Errorf("postgres continuous: unexpected protocol message %T", raw)
	}
}
