package primary

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block/index"
	telemetry "github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Index interface {
	component.Component
	index.Codec

	// Search returns item location by ID.
	Search(ctx context.Context, id uint64) (ref FeedRef, ok bool)
	// Add adds item location to the index.
	Add(ctx context.Context, id uint64, item FeedRef)
	// IDs returns all item IDs.
	IDs(ctx context.Context) (ids map[uint64]bool)
	// Count returns the number of feeds in the index.
	Count(ctx context.Context) (count uint32)
}

type Config struct{}

type Dependencies struct{}

var (
	headerMagicNumber = []byte{0x77, 0x79, 0x73, 0x20, 0x69, 0x73, 0x20,
		0x61, 0x77, 0x65, 0x73, 0x6f, 0x6d, 0x65, 0x00, 0x00}
	headerVersion = uint8(1)
)

type FeedRef struct {
	Chunk  uint32
	Offset uint64
	Time   time.Time
}

// --- Factory code block ---
type Factory component.Factory[Index, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Index, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (Index, error) {
				m := &mockIndex{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Index, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Index, error) {
	return &idx{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "FeedPrimaryIndex",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		m: make(map[uint64]FeedRef, 64),
	}, nil
}

// --- Implementation code block ---
type idx struct {
	*component.Base[Config, Dependencies]

	m  map[uint64]FeedRef
	mu sync.RWMutex
}

func (idx *idx) Search(ctx context.Context, id uint64) (ref FeedRef, ok bool) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "Search")...)
	defer func() { telemetry.End(ctx, nil) }()

	idx.mu.RLock()
	defer idx.mu.RUnlock()
	ref, ok = idx.m[id]

	return ref, ok
}

func (idx *idx) Add(ctx context.Context, id uint64, item FeedRef) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "Add")...)
	defer func() { telemetry.End(ctx, nil) }()

	idx.mu.Lock()
	defer idx.mu.Unlock()
	item.Time = item.Time.In(time.UTC)
	idx.m[id] = item
}

func (idx *idx) IDs(ctx context.Context) (ids map[uint64]bool) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "IDs")...)
	defer func() { telemetry.End(ctx, nil) }()

	idx.mu.RLock()
	defer idx.mu.RUnlock()
	result := make(map[uint64]bool, len(idx.m))
	for id := range idx.m {
		result[id] = true
	}

	return result
}

func (idx *idx) Count(ctx context.Context) (count uint32) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "Count")...)
	defer func() { telemetry.End(ctx, nil) }()

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return uint32(len(idx.m))
}

func (idx *idx) EncodeTo(ctx context.Context, w io.Writer) (err error) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "EncodeTo")...)
	defer func() { telemetry.End(ctx, err) }()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Write header.
	if _, err := w.Write(headerMagicNumber); err != nil {
		return errors.Wrap(err, "write header magic number")
	}
	if _, err := w.Write([]byte{headerVersion}); err != nil {
		return errors.Wrap(err, "write header version")
	}

	// Write map count.
	count := uint64(len(idx.m))
	if err := binary.Write(w, binary.LittleEndian, count); err != nil {
		return errors.Wrap(err, "write map count")
	}

	// Write all key-value pairs.
	for id, ref := range idx.m {
		// Write Key.
		if err := binary.Write(w, binary.LittleEndian, id); err != nil {
			return errors.Wrap(err, "write id")
		}

		// Write Value.
		if err := binary.Write(w, binary.LittleEndian, ref.Chunk); err != nil {
			return errors.Wrap(err, "write chunk")
		}
		if err := binary.Write(w, binary.LittleEndian, ref.Offset); err != nil {
			return errors.Wrap(err, "write offset")
		}
		if err := binary.Write(w, binary.LittleEndian, ref.Time.UnixNano()); err != nil {
			return errors.Wrap(err, "write time")
		}
	}

	return nil
}

func (idx *idx) DecodeFrom(ctx context.Context, r io.Reader) (err error) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "DecodeFrom")...)
	defer func() { telemetry.End(ctx, err) }()
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Read header.
	if err := idx.readHeader(r); err != nil {
		return errors.Wrap(err, "read header")
	}

	// Read map count.
	var count uint64
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "read map count")
	}
	idx.m = make(map[uint64]FeedRef, count)

	// Read all key-value pairs.
	for range count {
		id, ref, err := idx.readEntry(r)
		if err != nil {
			return errors.Wrap(err, "read entry")
		}
		idx.m[id] = ref
	}

	return nil
}

// readHeader reads and validates the index file header.
func (idx *idx) readHeader(r io.Reader) error {
	magicNumber := make([]byte, len(headerMagicNumber))
	if _, err := io.ReadFull(r, magicNumber); err != nil {
		return errors.Wrap(err, "read magic number")
	}
	if !bytes.Equal(magicNumber, headerMagicNumber) {
		return errors.New("invalid magic number")
	}

	versionByte := make([]byte, 1)
	if _, err := io.ReadFull(r, versionByte); err != nil {
		return errors.Wrap(err, "read version")
	}
	if versionByte[0] != headerVersion {
		return errors.New("invalid version")
	}

	return nil
}

// readEntry reads a single key-value pair (feed ID and FeedRef) from the reader.
func (idx *idx) readEntry(r io.Reader) (id uint64, ref FeedRef, err error) {
	// Read Key (ID).
	if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
		return 0, FeedRef{}, errors.Wrap(err, "read id")
	}

	// Read Value (FeedRef).
	if err := binary.Read(r, binary.LittleEndian, &ref.Chunk); err != nil {
		return 0, FeedRef{}, errors.Wrap(err, "read chunk")
	}
	if err := binary.Read(r, binary.LittleEndian, &ref.Offset); err != nil {
		return 0, FeedRef{}, errors.Wrap(err, "read offset")
	}
	var timestamp int64
	if err := binary.Read(r, binary.LittleEndian, &timestamp); err != nil {
		return 0, FeedRef{}, errors.Wrap(err, "read time")
	}
	ref.Time = time.Unix(0, timestamp).In(time.UTC)

	return id, ref, nil
}

type mockIndex struct {
	component.Mock
}

func (m *mockIndex) Search(ctx context.Context, id uint64) (ref FeedRef, ok bool) {
	args := m.Called(ctx, id)

	return args.Get(0).(FeedRef), args.Bool(1)
}

func (m *mockIndex) Add(ctx context.Context, id uint64, item FeedRef) {
	m.Called(ctx, id, item)
}

func (m *mockIndex) IDs(ctx context.Context) (ids map[uint64]bool) {
	args := m.Called(ctx)

	return args.Get(0).(map[uint64]bool)
}

func (m *mockIndex) Count(ctx context.Context) (count uint32) {
	args := m.Called(ctx)

	return args.Get(0).(uint32)
}

func (m *mockIndex) EncodeTo(ctx context.Context, w io.Writer) (err error) {
	args := m.Called(ctx, w)

	return args.Error(0)
}

func (m *mockIndex) DecodeFrom(ctx context.Context, r io.Reader) (err error) {
	args := m.Called(ctx, r)

	return args.Error(0)
}
