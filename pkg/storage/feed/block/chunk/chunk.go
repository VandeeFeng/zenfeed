// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package chunk

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edsrzf/mmap-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
	"github.com/vandeefeng/zenfeed/pkg/util/buffer"
	timeutil "github.com/vandeefeng/zenfeed/pkg/util/time"
)

// --- Interface code block ---

// File is the interface for a chunk file.
// Concurrent safe.
type File interface {
	component.Component

	// EnsureReadonly ensures the file is readonly (can not Append).
	// It should be fast when the file already is readonly.
	// It will ensure the writeonly related resources are closed,
	// and open the readonly related resources, such as mmap to save memory.
	EnsureReadonly(ctx context.Context) (err error)
	Count(ctx context.Context) (count uint32)

	// Append appends feeds to the file.
	// onSuccess is called when the feed is appended successfully (synchronously).
	// The offset is the offset of the feed in the file.
	// !!! It doesn't buffer the data between requests, so the caller should buffer the feeds to avoid high I/O.
	Append(ctx context.Context, feeds []*Feed, onSuccess func(feed *Feed, offset uint64) error) (err error)

	// Read reads a feed from the file.
	Read(ctx context.Context, offset uint64) (feed *Feed, err error)

	// Range ranges over all feeds in the file.
	Range(ctx context.Context, iter func(feed *Feed, offset uint64) (err error)) (err error)
}

// Config for a chunk file.
type Config struct {
	// Path is the path to the chunk file.
	// If the file does not exist, it will be created.
	// If the file exists, it will be reloaded.
	Path string
	// ReadonlyAtFirst indicates whether the file should be readonly at first.
	// If file of path does not exist, it cannot be true.
	ReadonlyAtFirst bool
}

func (c *Config) Validate() (fileExists bool, err error) {
	if c.Path == "" {
		return false, errors.New("path is required")
	}

	fi, err := os.Stat(c.Path)
	switch {
	case err == nil:
		if fi.IsDir() {
			return false, errors.New("path is a directory")
		}

		return true, nil

	case os.IsNotExist(err):
		if c.ReadonlyAtFirst {
			return false, errors.New("path does not exist")
		}

		return false, nil

	default:
		return false, errors.Wrap(err, "stat path")
	}
}

type Dependencies struct{}

// File struct.
var (
	headerBytes       = 64
	headerMagicNumber = []byte{0x77, 0x79, 0x73, 0x20, 0x69, 0x73, 0x20,
		0x61, 0x77, 0x65, 0x73, 0x6f, 0x6d, 0x65, 0x00, 0x00}
	headerMagicNumberBytes = 16
	headerVersionStart     = headerMagicNumberBytes
	headerVersion          = uint32(1)
	headerVersionBytes     = 4
	dataStart              = headerBytes

	header = func() []byte {
		b := make([]byte, headerBytes)
		copy(b[:headerMagicNumberBytes], headerMagicNumber)
		binary.LittleEndian.PutUint32(b[headerVersionStart:headerVersionStart+headerVersionBytes], headerVersion)

		return b
	}()
)

// Metrics.
var (
	modes     = []string{"readwrite", "readonly"}
	feedCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: model.AppName,
			Subsystem: "chunk",
			Name:      "feed_count",
			Help:      "Number of feeds in the chunk file.",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, "mode"},
	)
	byteSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: model.AppName,
			Subsystem: "chunk",
			Name:      "bytes",
			Help:      "Size of the chunk file.",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, "mode"},
	)
)

// --- Factory code block ---
type Factory component.Factory[File, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[File, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (File, error) {
				m := &mockFile{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[File, Config, Dependencies](new)
}

// new creates a new chunk file.
// It will create a new chunk file if the file that path points to does not exist.
// It will open the file if the file exists, and reload it.
// If readonlyAtFirst is true, it will open the file readonly.
func new(instance string, config *Config, dependencies Dependencies) (File, error) {
	fileExists, err := config.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	osFile, readWriteBuf, appendOffset, readonlyMmap, count, err := init0(fileExists, config)
	if err != nil {
		return nil, err
	}

	var rn atomic.Bool
	rn.Store(config.ReadonlyAtFirst)
	var cnt atomic.Uint32
	cnt.Store(count)

	return &file{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "FeedChunk",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		f:            osFile,
		readWriteBuf: readWriteBuf,
		appendOffset: appendOffset,
		readonlyMmap: readonlyMmap,
		readonly:     &rn,
		count:        &cnt,
	}, nil
}

func init0(
	fileExists bool,
	config *Config,
) (
	osFile *os.File,
	readWriteBuf *buffer.Bytes,
	appendOffset uint64,
	readonlyMmap mmap.MMap,
	count uint32,
	err error,
) {
	// Ensure file.
	if fileExists {
		osFile, err = loadFromExisting(config.Path, config.ReadonlyAtFirst)
		if err != nil {
			return nil, nil, 0, nil, 0, errors.Wrap(err, "load from existing")
		}

	} else { // Create new file.
		if config.ReadonlyAtFirst {
			return nil, nil, 0, nil, 0, errors.New("cannot create readonly file")
		}

		osFile, err = createNewOSFile(config.Path)
		if err != nil {
			return nil, nil, 0, nil, 0, errors.Wrap(err, "create new os file")
		}
	}

	// Setup for Read.
	readWriteBuf, count, err = validateOSFile(osFile)
	if err != nil {
		_ = osFile.Close()

		return nil, nil, 0, nil, 0, errors.Wrap(err, "validate os file")
	}

	if config.ReadonlyAtFirst {
		readWriteBuf = nil // Help GC.

		m, err := mmap.Map(osFile, mmap.RDONLY, 0)
		if err != nil {
			_ = osFile.Close()

			return nil, nil, 0, nil, 0, errors.Wrap(err, "mmap file")
		}

		readonlyMmap = m

	} else {
		appendOffset = uint64(readWriteBuf.Len())
	}

	return
}

func validateOSFile(f *os.File) (readWriteBuf *buffer.Bytes, count uint32, err error) {
	header, err := validateHeader(f)
	if err != nil {
		return nil, 0, errors.Wrap(err, "validate header")
	}
	readWriteBuf = &buffer.Bytes{B: header} // len(header) == cap(header).

	if _, err := f.Seek(int64(dataStart), io.SeekStart); err != nil {
		return nil, 0, errors.Wrap(err, "seek to data start")
	}
	tr := &trackReader{Reader: f}
	var lastSuccessReaded int

	var p Feed
	for {
		err := p.validateFrom(tr, readWriteBuf)
		switch {
		case err == nil:
			count++
			lastSuccessReaded = tr.Readed()

			continue

		case (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) ||
			errors.Is(err, errChecksumMismatch):

			// Truncate uncompleted feed if any.
			readWriteBuf.B = readWriteBuf.B[:lastSuccessReaded+len(header)]

			return readWriteBuf, count, nil

		default:
			return nil, 0, errors.Wrap(err, "validate payload")
		}
	}
}

func validateHeader(f *os.File) (header []byte, err error) {
	header = make([]byte, headerBytes)
	if _, err := f.ReadAt(header, 0); err != nil {
		return nil, errors.Wrap(err, "read header")
	}

	// Validate magic number.
	if !bytes.Equal(header[:headerMagicNumberBytes], headerMagicNumber) {
		return nil, errors.New("invalid magic number")
	}

	// Validate version.
	version := binary.LittleEndian.Uint32(header[headerVersionStart : headerVersionStart+headerVersionBytes])
	if version != headerVersion {
		return nil, errors.New("invalid version")
	}

	return header, nil
}

func loadFromExisting(path string, readonlyAtFirst bool) (osFile *os.File, err error) {
	flag := os.O_RDWR
	if readonlyAtFirst {
		flag = os.O_RDONLY
	}

	osFile, err = os.OpenFile(path, flag, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "open file")
	}

	return osFile, nil
}

func createNewOSFile(path string) (osFile *os.File, err error) {
	osFile, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "create file")
	}

	if _, err = osFile.Write(header); err != nil {
		_ = osFile.Close()

		return nil, errors.Wrap(err, "write header")
	}

	if err = osFile.Sync(); err != nil {
		_ = osFile.Close()

		return nil, errors.Wrap(err, "sync file")
	}

	return osFile, nil
}

// --- Implementation code block ---
type file struct {
	*component.Base[Config, Dependencies]

	f        *os.File
	count    *atomic.Uint32
	readonly *atomic.Bool

	mu sync.RWMutex

	// Only readwrite.
	readWriteBuf *buffer.Bytes
	appendOffset uint64

	// Only readonly.
	readonlyMmap mmap.MMap
}

func (f *file) Run() error {
	f.MarkReady()

	return timeutil.Tick(f.Context(), 30*time.Second, func() error {
		mode := "readwrite"
		sizeValue := f.appendOffset
		if f.readonly.Load() {
			mode = "readonly"
			sizeValue = uint64(len(f.readonlyMmap))
		}

		feedCount.WithLabelValues(append(f.TelemetryLabelsIDFields(), mode)...).Set(float64(f.Count(context.Background())))
		byteSize.WithLabelValues(append(f.TelemetryLabelsIDFields(), mode)...).Set(float64(sizeValue))
		for _, m := range modes {
			if m == mode {
				continue
			}
			feedCount.DeleteLabelValues(append(f.TelemetryLabelsIDFields(), m)...)
			byteSize.DeleteLabelValues(append(f.TelemetryLabelsIDFields(), m)...)
		}

		return nil
	})
}

func (f *file) Close() error {
	// Close Run().
	if err := f.Base.Close(); err != nil {
		return errors.Wrap(err, "closing base")
	}

	// Clean metrics.
	feedCount.DeletePartialMatch(f.TelemetryLabelsID())
	byteSize.DeletePartialMatch(f.TelemetryLabelsID())

	// Unmap if readonly.
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readonlyMmap != nil {
		if err := f.readonlyMmap.Unmap(); err != nil {
			return errors.Wrap(err, "unmap file")
		}
		f.readonlyMmap = nil
	}

	// Close file.
	if err := f.f.Close(); err != nil {
		return errors.Wrap(err, "close file")
	}
	f.f = nil
	f.appendOffset = 0

	return nil
}

func (f *file) EnsureReadonly(ctx context.Context) (err error) {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "EnsureReadonly")...)
	defer func() { telemetry.End(ctx, err) }()

	// Fast path - already readonly.
	if f.readonly.Load() {
		return nil
	}

	// Acquire write lock
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.readonly.Load() {
		return nil
	}

	// Clear readwrite resources.
	f.readWriteBuf = nil

	// Open mmap.
	m, err := mmap.Map(f.f, mmap.RDONLY, 0)
	if err != nil {
		return errors.Wrap(err, "mmap file")
	}

	// Update state.
	f.readonlyMmap = m
	f.readonly.Store(true)

	return nil
}

func (f *file) Count(ctx context.Context) uint32 {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "Count")...)
	defer func() { telemetry.End(ctx, nil) }()

	return f.count.Load()
}

func (f *file) Append(ctx context.Context, feeds []*Feed, onSuccess func(feed *Feed, offset uint64) error) (err error) {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "Append")...)
	defer func() { telemetry.End(ctx, err) }()

	f.mu.Lock()

	// Precheck.
	if f.readonly.Load() {
		f.mu.Unlock()

		return errors.New("file is readonly")
	}

	// Encode feeds into buffer.
	currentAppendOffset := f.appendOffset
	relativeOffsets, encodedBytesCount, err := f.encodeFeeds(feeds)
	if err != nil {
		f.readWriteBuf.B = f.readWriteBuf.B[:currentAppendOffset]
		f.mu.Unlock()

		return errors.Wrap(err, "encode feeds")
	}

	// Prepare for commit.
	encodedData := f.readWriteBuf.Bytes()[currentAppendOffset:]
	newAppendOffset := currentAppendOffset + uint64(encodedBytesCount)

	// Commit data and header to file.
	if err = f.commitAppendToFile(encodedData, currentAppendOffset); err != nil {
		f.readWriteBuf.B = f.readWriteBuf.B[:currentAppendOffset]
		f.mu.Unlock()

		return errors.Wrap(err, "commit append to file")
	}

	// Update internal state on successful commit.
	f.appendOffset = newAppendOffset
	f.count.Add(uint32(len(feeds)))
	f.mu.Unlock()

	// Call callbacks after releasing the lock.
	absoluteOffsets := make([]uint64, len(relativeOffsets))
	for i, relOff := range relativeOffsets {
		absoluteOffsets[i] = currentAppendOffset + relOff // Calculate absolute offsets based on append position.
	}
	if err := f.notifySuccess(feeds, absoluteOffsets, onSuccess); err != nil {
		return errors.Wrap(err, "notify success callbacks")
	}

	return nil
}

func (f *file) Read(ctx context.Context, offset uint64) (feed *Feed, err error) {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "Read")...)
	defer func() { telemetry.End(ctx, err) }()

	// Validate offset.
	if offset < uint64(dataStart) {
		return nil, errors.New("offset too small")
	}

	// Handle readonly mode.
	if f.readonly.Load() {
		if offset >= uint64(len(f.readonlyMmap)) {
			return nil, errors.New("offset too large")
		}
		feed, _, err = f.readFeed(ctx, f.readonlyMmap, offset)
		if err != nil {
			return nil, errors.Wrap(err, "read feed")
		}

		return feed, nil
	}

	// Handle readwrite mode.
	f.mu.RLock()
	defer f.mu.RUnlock()
	if offset >= f.appendOffset {
		return nil, errors.New("offset too large")
	}

	feed, _, err = f.readFeed(ctx, f.readWriteBuf.Bytes(), offset)
	if err != nil {
		return nil, errors.Wrap(err, "read feed")
	}

	return feed, nil
}

func (f *file) Range(ctx context.Context, iter func(feed *Feed, offset uint64) error) (err error) {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "Range")...)
	defer func() { telemetry.End(ctx, err) }()

	// Handle readonly mode.
	if f.readonly.Load() {
		// Start from data section.
		offset := uint64(dataStart)
		for offset < uint64(len(f.readonlyMmap)) {
			feed, n, err := f.readFeed(ctx, f.readonlyMmap, offset)
			if err != nil {
				return errors.Wrap(err, "read feed")
			}
			if err := iter(feed, offset); err != nil {
				return errors.Wrap(err, "iterate feed")
			}

			// Move to next feed.
			offset += uint64(n) // G115: Safe conversion as n is uint32
		}

		return nil
	}

	// Handle readwrite mode.
	f.mu.RLock()
	defer f.mu.RUnlock()
	data := f.readWriteBuf.Bytes()
	offset := uint64(dataStart)
	for offset < f.appendOffset { // appendOffset is already checked/maintained correctly.
		feed, n, err := f.readFeed(ctx, data, offset)
		if err != nil {
			return errors.Wrap(err, "read feed")
		}
		if err := iter(feed, offset); err != nil {
			return errors.Wrap(err, "iterate feed")
		}

		// Move to next feed.
		offset += uint64(n)
	}

	return nil
}

const estimatedFeedSize = 4 * 1024

// encodeFeeds encodes a slice of feeds into the internal readWriteBuf.
// It returns the relative offsets of each feed within the newly added data,
// the total number of bytes encoded, and any error encountered.
func (f *file) encodeFeeds(feeds []*Feed) (relativeOffsets []uint64, encodedBytesCount int, err error) {
	relativeOffsets = make([]uint64, len(feeds))
	startOffset := f.readWriteBuf.Len()

	f.readWriteBuf.EnsureRemaining(estimatedFeedSize * len(feeds))

	for i, feed := range feeds {
		currentOffsetInBuf := f.readWriteBuf.Len()
		relativeOffsets[i] = uint64(currentOffsetInBuf - startOffset)
		if err := feed.encodeTo(f.readWriteBuf); err != nil {
			return nil, 0, errors.Wrapf(err, "encode feed %d", i)
		}
	}

	encodedBytesCount = f.readWriteBuf.Len() - startOffset

	return relativeOffsets, encodedBytesCount, nil
}

// commitAppendToFile writes the encoded data and updated header to the file and syncs.
func (f *file) commitAppendToFile(data []byte, currentAppendOffset uint64) error {
	// Append data.
	if _, err := f.f.WriteAt(data, int64(currentAppendOffset)); err != nil {
		// Data might be partially written.
		// We will overwrite it in the next append.
		return errors.Wrap(err, "write feeds")
	}

	// Sync file to persist changes.
	if err := f.f.Sync(); err != nil {
		return errors.Wrap(err, "sync file")
	}

	return nil
}

// notifySuccess calls the onSuccess callback for each successfully appended feed.
func (f *file) notifySuccess(
	feeds []*Feed,
	absoluteOffsets []uint64,
	onSuccess func(feed *Feed, offset uint64) error,
) error {
	if onSuccess == nil {
		return nil
	}
	for i, feed := range feeds {
		if err := onSuccess(feed, absoluteOffsets[i]); err != nil {
			// Return the first error encountered during callbacks.
			return errors.Wrapf(err, "on success callback for feed %d", i)
		}
	}

	return nil
}

func (f *file) readFeed(ctx context.Context, data []byte, offset uint64) (feed *Feed, length int, err error) {
	ctx = telemetry.StartWith(ctx, append(f.TelemetryLabels(), telemetrymodel.KeyOperation, "readFeed")...)
	defer func() { telemetry.End(ctx, err) }()

	// Prepare reader.
	r := io.NewSectionReader(bytes.NewReader(data), int64(offset), int64(uint64(len(data))-offset))
	tr := &trackReader{Reader: r}

	// Decode feed.
	feed = &Feed{Feed: &model.Feed{}}
	if err = feed.decodeFrom(tr); err != nil {
		return nil, 0, errors.Wrap(err, "decode feed")
	}

	return feed, tr.Readed(), nil
}

type trackReader struct {
	io.Reader
	length int
}

func (r *trackReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.length += n

	return
}

func (r *trackReader) Readed() int {
	return r.length
}

type mockFile struct {
	component.Mock
}

func (m *mockFile) Run() error {
	args := m.Called()

	return args.Error(0)
}

func (m *mockFile) Ready() <-chan struct{} {
	args := m.Called()

	return args.Get(0).(<-chan struct{})
}

func (m *mockFile) Close() error {
	args := m.Called()

	return args.Error(0)
}

func (m *mockFile) Append(ctx context.Context, feeds []*Feed, onSuccess func(feed *Feed, offset uint64) error) error {
	args := m.Called(ctx, feeds, onSuccess)

	return args.Error(0)
}

func (m *mockFile) Read(ctx context.Context, offset uint64) (*Feed, error) {
	args := m.Called(ctx, offset)

	return args.Get(0).(*Feed), args.Error(1)
}

func (m *mockFile) Range(ctx context.Context, iter func(feed *Feed, offset uint64) error) error {
	args := m.Called(ctx, iter)

	return args.Error(0)
}

func (m *mockFile) Count(ctx context.Context) uint32 {
	args := m.Called(ctx)

	return args.Get(0).(uint32)
}

func (m *mockFile) EnsureReadonly(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}
