package analyze

import (
	"errors"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// Records are stored in the protocol buffers wire format, written and read by
// hand with protowire rather than through generated code, which keeps protoc
// out of the build and is several times faster. The schema is:
//
//	message Record {
//	  uint32 format = 1;
//	  string type = 2;
//	  int32 flag = 3;
//	  int64 size = 4;
//	  int64 usage = 5;
//	  int64 own_usage = 6;
//	  int64 item_count = 7;
//	  int64 mtime_sec = 8;   // present only for a set mtime
//	  int32 mtime_nsec = 9;
//	  repeated Entry entries = 10;
//	  uint64 dev = 11;
//	}
//
//	message Entry {
//	  string name = 1;
//	  bool dir = 2;
//	  int32 flag = 3;
//	  int64 size = 4;
//	  int64 usage = 5;
//	  int64 shared_usage = 6;
//	  int64 mtime_sec = 7;   // present only for a set mtime
//	  int32 mtime_nsec = 8;
//	  uint64 mli = 9;
//	  string symlink = 10;
//	}
//
// Unknown fields are skipped, so fields can be added without a new format.
type (
	recordField protowire.Number
	entryField  protowire.Number
)

const (
	recordFormat    recordField = 1
	recordType      recordField = 2
	recordFlag      recordField = 3
	recordSize      recordField = 4
	recordUsage     recordField = 5
	recordOwnUsage  recordField = 6
	recordItemCount recordField = 7
	recordMtimeSec  recordField = 8
	recordMtimeNsec recordField = 9
	recordEntries   recordField = 10
	recordDev       recordField = 11
)

const (
	entryName        entryField = 1
	entryDir         entryField = 2
	entryFlag        entryField = 3
	entrySize        entryField = 4
	entryUsage       entryField = 5
	entrySharedUsage entryField = 6
	entryMtimeSec    entryField = 7
	entryMtimeNsec   entryField = 8
	entryMli         entryField = 9
	entrySymlink     entryField = 10
)

type field interface{ recordField | entryField }

var errMalformedRecord = errors.New("malformed record")

// marshal returns the record in the wire format.
func (r *storedRecord) marshal() []byte {
	b := make([]byte, 0, 64+48*len(r.Entries))
	b = appendVarint(b, recordFormat, uint64(r.Format))
	b = appendString(b, recordType, r.Type)
	b = appendVarint(b, recordFlag, uint64(r.Flag))
	b = appendVarint(b, recordSize, uint64(r.Size))
	b = appendVarint(b, recordUsage, uint64(r.Usage))
	b = appendVarint(b, recordOwnUsage, uint64(r.OwnUsage))
	b = appendVarint(b, recordItemCount, uint64(r.ItemCount))
	b = appendTime(b, recordMtimeSec, recordMtimeNsec, r.Mtime)
	b = appendVarint(b, recordDev, r.Dev)

	var entry []byte
	for i := range r.Entries {
		entry = r.Entries[i].marshal(entry[:0])
		b = protowire.AppendTag(b, protowire.Number(recordEntries), protowire.BytesType)
		b = protowire.AppendBytes(b, entry)
	}
	return b
}

func (e *storedEntry) marshal(b []byte) []byte {
	b = appendString(b, entryName, e.Name)
	if e.Dir {
		b = appendVarint(b, entryDir, 1)
	}
	b = appendVarint(b, entryFlag, uint64(e.Flag))
	b = appendVarint(b, entrySize, uint64(e.Size))
	b = appendVarint(b, entryUsage, uint64(e.Usage))
	b = appendVarint(b, entrySharedUsage, uint64(e.SharedUsage))
	b = appendTime(b, entryMtimeSec, entryMtimeNsec, e.Mtime)
	b = appendVarint(b, entryMli, e.Mli)
	return appendString(b, entrySymlink, e.Symlink)
}

// unmarshal reads the record from b, which it does not retain.
func (r *storedRecord) unmarshal(b []byte) error {
	count := 0
	if err := eachField(b, func(num protowire.Number, _ uint64, _ []byte) error {
		if recordField(num) == recordEntries {
			count++
		}
		return nil
	}); err != nil {
		return err
	}
	r.Entries = make([]storedEntry, 0, count)

	var mtime timeFields
	err := eachField(b, func(num protowire.Number, v uint64, bytes []byte) error {
		switch recordField(num) {
		case recordFormat:
			r.Format = int(v)
		case recordType:
			r.Type = string(bytes)
		case recordFlag:
			r.Flag = rune(int32(v))
		case recordSize:
			r.Size = int64(v)
		case recordUsage:
			r.Usage = int64(v)
		case recordOwnUsage:
			r.OwnUsage = int64(v)
		case recordItemCount:
			r.ItemCount = int64(v)
		case recordMtimeSec:
			mtime.setSec(v)
		case recordMtimeNsec:
			mtime.nsec = int64(int32(v))
		case recordDev:
			r.Dev = v
		case recordEntries:
			var entry storedEntry
			if err := entry.unmarshal(bytes); err != nil {
				return err
			}
			r.Entries = append(r.Entries, entry)
		}
		return nil
	})
	r.Mtime = mtime.time()
	return err
}

func (e *storedEntry) unmarshal(b []byte) error {
	var mtime timeFields
	err := eachField(b, func(num protowire.Number, v uint64, bytes []byte) error {
		switch entryField(num) {
		case entryName:
			e.Name = string(bytes)
		case entryDir:
			e.Dir = v != 0
		case entryFlag:
			e.Flag = rune(int32(v))
		case entrySize:
			e.Size = int64(v)
		case entryUsage:
			e.Usage = int64(v)
		case entrySharedUsage:
			e.SharedUsage = int64(v)
		case entryMtimeSec:
			mtime.setSec(v)
		case entryMtimeNsec:
			mtime.nsec = int64(int32(v))
		case entryMli:
			e.Mli = v
		case entrySymlink:
			e.Symlink = string(bytes)
		}
		return nil
	})
	e.Mtime = mtime.time()
	return err
}

// eachField calls fn with each varint and length-delimited field of the
// message in b, skipping fields of other wire types.
func eachField(b []byte, fn func(num protowire.Number, v uint64, bytes []byte) error) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return errMalformedRecord
		}
		b = b[n:]

		known := true
		var (
			v     uint64
			bytes []byte
		)
		switch typ {
		case protowire.VarintType:
			v, n = protowire.ConsumeVarint(b)
		case protowire.BytesType:
			bytes, n = protowire.ConsumeBytes(b)
		case protowire.Fixed32Type, protowire.Fixed64Type, protowire.StartGroupType, protowire.EndGroupType:
			known = false
			n = protowire.ConsumeFieldValue(num, typ, b)
		default:
			return errMalformedRecord
		}
		if n < 0 {
			return errMalformedRecord
		}
		b = b[n:]

		if known {
			if err := fn(num, v, bytes); err != nil {
				return err
			}
		}
	}
	return nil
}

// appendVarint appends a varint field, omitting it when zero as proto3 does.
func appendVarint[F field](b []byte, num F, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, protowire.Number(num), protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func appendString[F field](b []byte, num F, s string) []byte {
	if s == "" {
		return b
	}
	b = protowire.AppendTag(b, protowire.Number(num), protowire.BytesType)
	return protowire.AppendString(b, s)
}

// appendTime appends t as seconds and nanoseconds since the Unix epoch. The
// seconds are written even when zero, so that the epoch itself is told apart
// from an unset time, which is omitted.
func appendTime[F field](b []byte, sec, nsec F, t time.Time) []byte {
	if t.IsZero() {
		return b
	}
	b = protowire.AppendTag(b, protowire.Number(sec), protowire.VarintType)
	b = protowire.AppendVarint(b, uint64(t.Unix()))
	return appendVarint(b, nsec, uint64(t.Nanosecond()))
}

// timeFields collects a time written by appendTime.
type timeFields struct {
	set       bool
	sec, nsec int64
}

func (t *timeFields) setSec(v uint64) {
	t.set = true
	t.sec = int64(v)
}

func (t *timeFields) time() time.Time {
	if !t.set {
		return time.Time{}
	}
	return time.Unix(t.sec, t.nsec)
}
