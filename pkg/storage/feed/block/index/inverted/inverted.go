package inverted

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"maps"
	"sync"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block/index"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
	binaryutil "github.com/vandeefeng/zenfeed/pkg/util/binary"
)

// --- Interface code block ---
type Index interface {
	component.Component
	index.Codec

	// Search returns item IDs matching the given label and value.
	Search(ctx context.Context, label string, eq bool, value string) (ids map[uint64]struct{})
	// Add adds item to the index.
	// If label or value in labels is empty, it will be ignored.
	// If value is too long, it will be ignored,
	// because does not support regex search, so long value is not useful.
	Add(ctx context.Context, id uint64, labels model.Labels)
}

type Config struct{}

type Dependencies struct{}

const (
	maxLabelValueLength = 64
)

var (
	headerMagicNumber = []byte{0x77, 0x79, 0x73, 0x20, 0x69, 0x73, 0x20,
		0x61, 0x77, 0x65, 0x73, 0x6f, 0x6d, 0x65, 0x00, 0x00}
	headerVersion = uint8(1)
)

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
			Name:         "FeedInvertedIndex",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		m:   make(map[string]map[string]map[uint64]struct{}, 64),
		ids: make(map[uint64]struct{}, 64),
	}, nil
}

// --- Implementation code block ---
type idx struct {
	*component.Base[Config, Dependencies]

	// Label -> values -> ids.
	m map[string]map[string]map[uint64]struct{}
	// All ids.
	ids map[uint64]struct{}
	mu  sync.RWMutex
}

func (idx *idx) Search(ctx context.Context, label string, eq bool, value string) (ids map[uint64]struct{}) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "Search")...)
	defer func() { telemetry.End(ctx, nil) }()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if value == "" {
		return idx.searchEmptyValue(label, eq)
	}

	return idx.searchNonEmptyValue(label, eq, value)
}

func (idx *idx) Add(ctx context.Context, id uint64, labels model.Labels) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "Add")...)
	defer func() { telemetry.End(ctx, nil) }()
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Add all labels.
	for _, label := range labels {
		if label.Key == "" || label.Value == "" {
			continue
		}
		if len(label.Value) > maxLabelValueLength {
			continue
		}

		if _, ok := idx.m[label.Key]; !ok {
			idx.m[label.Key] = make(map[string]map[uint64]struct{})
		}
		if _, ok := idx.m[label.Key][label.Value]; !ok {
			idx.m[label.Key][label.Value] = make(map[uint64]struct{})
		}
		idx.m[label.Key][label.Value][id] = struct{}{}
	}

	// Add to ids.
	idx.ids[id] = struct{}{}
}

func (idx *idx) EncodeTo(ctx context.Context, w io.Writer) (err error) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "EncodeTo")...)
	defer func() { telemetry.End(ctx, err) }()
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if err := idx.writeHeader(w); err != nil {
		return errors.Wrap(err, "write header")
	}

	if err := idx.writeLabels(w); err != nil {
		return errors.Wrap(err, "write labels")
	}

	return nil
}

// DecodeFrom decodes the index from the given reader.
func (idx *idx) DecodeFrom(ctx context.Context, r io.Reader) (err error) {
	ctx = telemetry.StartWith(ctx, append(idx.TelemetryLabels(), telemetrymodel.KeyOperation, "DecodeFrom")...)
	defer func() { telemetry.End(ctx, err) }()
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Read header.
	if err := idx.readHeader(r); err != nil {
		return errors.Wrap(err, "read header")
	}

	// Read labels.
	if err := idx.readLabels(r); err != nil {
		return errors.Wrap(err, "read labels")
	}

	return nil
}

// searchEmptyValue handles the search logic when the target value is empty.
// If eq is true, it returns IDs that *do not* have the given label.
// If eq is false, it returns IDs that *do* have the given label (with any value).
func (idx *idx) searchEmptyValue(label string, eq bool) map[uint64]struct{} {
	// Find all IDs associated with the given label, regardless of value.
	idsWithLabel := make(map[uint64]struct{})
	if values, ok := idx.m[label]; ok {
		for _, ids := range values {
			for id := range ids {
				idsWithLabel[id] = struct{}{}
			}
		}
	}

	// If not equal (!eq), return the IDs that have the label.
	if !eq {
		return idsWithLabel
	}

	// If equal (eq), return IDs that *do not* have the label.
	// Start with all known IDs and remove those that have the label.
	resultIDs := maps.Clone(idx.ids)
	for id := range idsWithLabel {
		delete(resultIDs, id)
	}

	return resultIDs
}

// searchNonEmptyValue handles the search logic when the target value is not empty.
// If eq is true, it returns IDs that have the exact label-value pair.
// If eq is false, it returns IDs that *do not* have the exact label-value pair.
func (idx *idx) searchNonEmptyValue(label string, eq bool, value string) map[uint64]struct{} {
	// Get the map of values for the given label.
	values, labelExists := idx.m[label]

	// If equal (eq), find the exact match.
	if eq {
		if !labelExists {
			return make(map[uint64]struct{}) // Label doesn't exist.
		}
		ids, valueExists := values[value]
		if !valueExists {
			return make(map[uint64]struct{}) // Value doesn't exist for this label.
		}

		// Return a clone to prevent modification of the underlying index data.
		return maps.Clone(ids)
	}

	// If not equal (!eq), return IDs that *do not* have this specific label-value pair.
	// Start with all known IDs.
	resultIDs := maps.Clone(idx.ids)
	if labelExists {
		// If the specific label-value pair exists, remove its associated IDs.
		if matchingIDs, valueExists := values[value]; valueExists {
			for id := range matchingIDs {
				delete(resultIDs, id)
			}
		}
	}

	return resultIDs
}

func (idx *idx) writeHeader(w io.Writer) error {
	if _, err := w.Write(headerMagicNumber); err != nil {
		return errors.Wrap(err, "write header magic number")
	}
	if _, err := w.Write([]byte{headerVersion}); err != nil {
		return errors.Wrap(err, "write header version")
	}

	return nil
}

func (idx *idx) writeLabels(w io.Writer) error {
	// Write total unique ID count.
	idCount := uint32(len(idx.ids))
	if err := binary.Write(w, binary.LittleEndian, idCount); err != nil {
		return errors.Wrap(err, "write total id count")
	}

	// Write label count.
	labelCount := uint32(len(idx.m))
	if err := binary.Write(w, binary.LittleEndian, labelCount); err != nil {
		return errors.Wrap(err, "write label count")
	}

	// Write each label and its associated value entries.
	for label, values := range idx.m {
		if err := idx.writeLabelEntry(w, label, values); err != nil {
			return errors.Wrap(err, "write label entry")
		}
	}

	return nil
}

// writeLabelEntry writes a single label, its value count, and then calls writeValueEntry for each value.
func (idx *idx) writeLabelEntry(w io.Writer, label string, values map[string]map[uint64]struct{}) error {
	// Write label string.
	if err := binaryutil.WriteString(w, label); err != nil {
		return errors.Wrap(err, "write label")
	}

	// Write value count for this label.
	valueCount := uint32(len(values))
	if err := binary.Write(w, binary.LittleEndian, valueCount); err != nil {
		return errors.Wrap(err, "write value count for label")
	}

	// Write each value and its associated IDs.
	for value, ids := range values {
		if err := idx.writeValueEntry(w, value, ids); err != nil {
			return errors.Wrap(err, "write value entry")
		}
	}

	return nil
}

// writeValueEntry writes a single value, its ID count, and then writes each associated ID.
func (idx *idx) writeValueEntry(w io.Writer, value string, ids map[uint64]struct{}) error {
	// Write value string.
	if err := binaryutil.WriteString(w, value); err != nil {
		return errors.Wrap(err, "write value")
	}

	// Write ID count for this label-value pair.
	idCount := uint32(len(ids))
	if err := binary.Write(w, binary.LittleEndian, idCount); err != nil {
		return errors.Wrap(err, "write id count for value")
	}

	// Write each associated ID.
	for id := range ids {
		if err := binary.Write(w, binary.LittleEndian, id); err != nil {
			return errors.Wrap(err, "write id")
		}
	}

	return nil
}

func (idx *idx) readHeader(r io.Reader) error {
	magicNumber := make([]byte, len(headerMagicNumber))
	if _, err := io.ReadFull(r, magicNumber); err != nil {
		return errors.Wrap(err, "read header magic number")
	}
	if !bytes.Equal(magicNumber, headerMagicNumber) {
		return errors.New("invalid magic number")
	}

	versionByte := make([]byte, 1)
	if _, err := io.ReadFull(r, versionByte); err != nil {
		return errors.Wrap(err, "read header version")
	}
	if versionByte[0] != headerVersion {
		return errors.New("invalid version")
	}

	return nil
}

func (idx *idx) readLabels(r io.Reader) error {
	// Read total unique ID count (used for pre-allocation).
	var totalIDCount uint32
	if err := binary.Read(r, binary.LittleEndian, &totalIDCount); err != nil {
		return errors.Wrap(err, "read total id count")
	}
	idx.ids = make(map[uint64]struct{}, totalIDCount) // Pre-allocate ids map.

	// Read label count.
	var labelCount uint32
	if err := binary.Read(r, binary.LittleEndian, &labelCount); err != nil {
		return errors.Wrap(err, "read label count")
	}
	idx.m = make(map[string]map[string]map[uint64]struct{}, labelCount) // Pre-allocate labels map.

	// Read each label and its associated value entries.
	for range labelCount {
		if err := idx.readLabelEntry(r); err != nil {
			return errors.Wrap(err, "read label entry")
		}
	}

	return nil
}

// readLabelEntry reads a single label, its value count, and then calls readValueEntry for each value.
func (idx *idx) readLabelEntry(r io.Reader) error {
	// Read label string.
	label, err := binaryutil.ReadString(r)
	if err != nil {
		return errors.Wrap(err, "read label")
	}

	// Read value count for this label.
	var valueCount uint32
	if err := binary.Read(r, binary.LittleEndian, &valueCount); err != nil {
		return errors.Wrap(err, "read value count for label")
	}
	idx.m[label] = make(map[string]map[uint64]struct{}, valueCount) // Pre-allocate values map for this label.

	// Read each value and its associated IDs.
	for range valueCount {
		if err := idx.readValueEntry(r, label); err != nil {
			return errors.Wrap(err, "read value entry")
		}
	}

	return nil
}

// readValueEntry reads a single value, its ID count, and then reads each associated ID, populating the index maps.
func (idx *idx) readValueEntry(r io.Reader, label string) error {
	// Read value string.
	value, err := binaryutil.ReadString(r)
	if err != nil {
		return errors.Wrap(err, "read value")
	}

	// Read ID count for this label-value pair.
	var idCount uint32
	if err := binary.Read(r, binary.LittleEndian, &idCount); err != nil {
		return errors.Wrap(err, "read id count for value")
	}
	idx.m[label][value] = make(map[uint64]struct{}, idCount) // Pre-allocate ids map for this label-value.

	// Read each associated ID.
	for range idCount {
		var id uint64
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			return errors.Wrap(err, "read id")
		}
		idx.m[label][value][id] = struct{}{}
		idx.ids[id] = struct{}{} // Add to the global set of IDs.
	}

	return nil
}

type mockIndex struct {
	component.Mock
}

func (m *mockIndex) Search(ctx context.Context, label string, eq bool, value string) (ids map[uint64]struct{}) {
	args := m.Called(ctx, label, eq, value)

	return args.Get(0).(map[uint64]struct{})
}

func (m *mockIndex) Add(ctx context.Context, id uint64, labels model.Labels) {
	m.Called(ctx, id, labels)
}

func (m *mockIndex) EncodeTo(ctx context.Context, w io.Writer) (err error) {
	args := m.Called(ctx, w)

	return args.Error(0)
}

func (m *mockIndex) DecodeFrom(ctx context.Context, r io.Reader) (err error) {
	args := m.Called(ctx, r)

	return args.Error(0)
}
