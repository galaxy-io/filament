package sink

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"unicode/utf8"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
)

const maxPartitionKeyChars = 256

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// encoded is payload accounting for one batch: CRC32C over concatenated JSON
// record bodies in input order, and their total byte length.
type encoded struct {
	stream string
	crc    uint32
	bytes  int64
}

// encode validates the complete batch before its first destination effect.
func (s *Sink) encode(b *arrowbatch.Batch) ([]types.PutRecordsRequestEntry, encoded, error) {
	stream, err := s.cfg.route(b.Resource)
	if err != nil {
		return nil, encoded{}, err
	}
	if b.Operations().Len() != 0 && b.Operations().Len() != b.NumRows() {
		return nil, encoded{}, errors.New("kinesis sink: operation count differs from row count")
	}
	rows := b.Rows()
	var keys *array.String
	if s.cfg.keyField != "" {
		keys, err = keyColumn(rows, s.cfg.keyField)
		if err != nil {
			return nil, encoded{}, err
		}
	}
	enc := jsonencoder.NewEncoder(rows.Schema())
	entries := make([]types.PutRecordsRequestEntry, 0, b.NumRows())
	acc := encoded{stream: stream}
	for i := range b.NumRows() {
		op := b.Op(i)
		if op != filament.OpInsert && op != filament.OpUpdate && op != filament.OpDelete {
			return nil, encoded{}, fmt.Errorf("kinesis sink: unsupported operation %d", op)
		}
		data := enc.AppendRow(nil, rows, i)
		if !json.Valid(data) {
			return nil, encoded{}, fmt.Errorf("kinesis sink: invalid JSON in row %d", i)
		}
		key := partitionKey(keys, i, data)
		if utf8.RuneCountInString(key) > maxPartitionKeyChars {
			return nil, encoded{}, fmt.Errorf("kinesis sink: row %d partition key exceeds %d characters", i, maxPartitionKeyChars)
		}
		if int64(len(data)+len(key)) > s.cfg.maxRecordBytes {
			return nil, encoded{}, fmt.Errorf("kinesis sink: row %d exceeds max_record_bytes %d", i, s.cfg.maxRecordBytes)
		}
		acc.crc = crc32.Update(acc.crc, crcTable, data)
		acc.bytes += int64(len(data))
		entries = append(entries, types.PutRecordsRequestEntry{Data: data, PartitionKey: aws.String(key)})
	}
	return entries, acc, nil
}

func keyColumn(rows arrow.RecordBatch, name string) (*array.String, error) {
	indices := rows.Schema().FieldIndices(name)
	if len(indices) != 1 {
		return nil, fmt.Errorf("kinesis sink: expected one %s column", name)
	}
	col, ok := rows.Column(indices[0]).(*array.String)
	if !ok {
		return nil, fmt.Errorf("kinesis sink: key_field %s must be a string column", name)
	}
	return col, nil
}

// partitionKey spreads rows without a key across shards by record checksum.
func partitionKey(keys *array.String, i int, data []byte) string {
	if keys != nil && !keys.IsNull(i) && keys.Value(i) != "" {
		return keys.Value(i)
	}
	return strconv.FormatUint(uint64(crc32.Checksum(data, crcTable)), 16)
}
