package chunk

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/model"
	binaryutil "github.com/vandeefeng/zenfeed/pkg/util/binary"
	"github.com/vandeefeng/zenfeed/pkg/util/buffer"
)

const (
	// feedHeaderSize is the size of the record header (length + checksum).
	feedHeaderSize = 8 // uint32 length + uint32 checksum
)

var (
	errChecksumMismatch = errors.New("checksum mismatch")

	crc32Table = crc32.MakeTable(crc32.IEEE)
)

// Feed is the feed model in the chunk file.
type Feed struct {
	*model.Feed
	Vectors [][]float32
}

// encodeTo encodes the Feed into the provided buffer, including a length prefix and checksum.
// It writes the record structure: [payloadLen(uint32)][checksum(uint32)][payload...].
func (f *Feed) encodeTo(buf *buffer.Bytes) error {
	buf.EnsureRemaining(4 * 1024)

	// 1. Reserve space for length and checksum.
	startOffset := buf.Len()
	headerPos := buf.Len()                   // Position where header starts.
	buf.B = buf.B[:headerPos+feedHeaderSize] // Extend buffer to include header space.
	payloadStartOffset := buf.Len()          // Position where payload starts.

	// 2. Encode the actual payload.
	if err := f.encodePayload(buf); err != nil {
		// If payload encoding fails, revert the buffer to its initial state.
		buf.B = buf.B[:startOffset]

		return errors.Wrap(err, "encode payload")
	}
	payloadEndOffset := buf.Len()

	// 3. Calculate payload length and checksum.
	payloadLen := uint32(payloadEndOffset - payloadStartOffset)
	payloadSlice := buf.Bytes()[payloadStartOffset:payloadEndOffset]
	checksum := crc32.Checksum(payloadSlice, crc32Table)

	// 4. Write the actual length and checksum into the reserved space.
	binary.LittleEndian.PutUint32(buf.Bytes()[headerPos:headerPos+4], payloadLen)
	binary.LittleEndian.PutUint32(buf.Bytes()[headerPos+4:headerPos+8], checksum)

	return nil
}

// encodePayload encodes the core fields (ID, Time, Labels, Vectors) into the buffer.
func (f *Feed) encodePayload(w io.Writer) error {
	// Write ID.
	if err := binaryutil.WriteUint64(w, f.ID); err != nil {
		return errors.Wrap(err, "write id")
	}

	// Write time.
	if err := binaryutil.WriteUint64(w, uint64(f.Time.UnixNano())); err != nil {
		return errors.Wrap(err, "write time")
	}

	// Write labels.
	if err := f.encodeLabels(w); err != nil {
		return errors.Wrap(err, "encode labels")
	}

	// Write vectors.
	if err := f.encodeVectors(w); err != nil {
		return errors.Wrap(err, "encode vectors")
	}

	return nil
}

// encodeLabels writes the label data to the writer.
func (f *Feed) encodeLabels(w io.Writer) error {
	labelsLen := uint32(len(f.Labels))
	if len(f.Labels) > math.MaxUint32 {
		return errors.New("too many labels")
	}
	if err := binaryutil.WriteUint32(w, labelsLen); err != nil {
		return errors.Wrap(err, "write labels count")
	}
	for i, label := range f.Labels {
		if err := binaryutil.WriteString(w, label.Key); err != nil {
			return errors.Wrapf(err, "write label key index %d", i)
		}
		if err := binaryutil.WriteString(w, label.Value); err != nil {
			return errors.Wrapf(err, "write label value index %d", i)
		}
	}

	return nil
}

// encodeVectors writes the vector data to the writer.
func (f *Feed) encodeVectors(w io.Writer) error {
	vectorCount := uint32(len(f.Vectors))
	if len(f.Vectors) > math.MaxUint32 {
		return errors.New("too many vectors")
	}
	if err := binaryutil.WriteUint32(w, vectorCount); err != nil {
		return errors.Wrap(err, "write vectors count")
	}
	if vectorCount == 0 {
		return nil // Nothing more to write if there are no vectors.
	}

	// Write dimension.
	dimension := uint32(len(f.Vectors[0]))
	if len(f.Vectors[0]) > math.MaxUint32 {
		return errors.New("vector dimension exceeds maximum uint32")
	}
	if err := binaryutil.WriteUint32(w, dimension); err != nil {
		return errors.Wrap(err, "write vector dimension")
	}

	// Write vector data.
	var floatBuf [4]byte
	for i, vec := range f.Vectors {
		// Ensure vector has the correct dimension.
		if uint32(len(vec)) != dimension {
			return errors.Errorf("vector %d has inconsistent dimension %d, expected %d", i, len(vec), dimension)
		}

		for _, val := range vec { // Avoid using binary.Write for performance.
			bits := math.Float32bits(val)
			binary.LittleEndian.PutUint32(floatBuf[:], bits)
			if _, err := w.Write(floatBuf[:]); err != nil {
				return errors.Wrapf(err, "write for vector %d, value %f", i, val)
			}
		}
	}

	return nil
}

func (f *Feed) validateFrom(r io.Reader, buf *buffer.Bytes) (err error) {
	// 1. Read header (length and checksum).
	var payloadLen, expectedChecksum uint32
	startOffset := buf.Len()
	if _, err := io.CopyN(buf, r, feedHeaderSize); err != nil {
		return errors.Wrap(err, "read header")
	}
	payloadLen = binary.LittleEndian.Uint32(buf.B[startOffset : startOffset+4])
	expectedChecksum = binary.LittleEndian.Uint32(buf.B[startOffset+4:])

	// 2. Read payload, calculate checksum simultaneously.
	buf.EnsureRemaining(int(payloadLen))
	limitedReader := io.LimitReader(r, int64(payloadLen))
	checksumWriter := crc32.New(crc32Table)
	teeReader := io.TeeReader(limitedReader, checksumWriter)

	// Read the exact payload length into the buffer.
	if _, err := io.CopyN(buf, teeReader, int64(payloadLen)); err != nil {
		// EOF, may be writing not complete.
		return errors.Wrap(err, "read payload")
	}

	// 3. Verify checksum.
	calculatedChecksum := checksumWriter.Sum32()
	if calculatedChecksum != expectedChecksum {
		return errors.Wrapf(errChecksumMismatch, "expected %x, got %x", expectedChecksum, calculatedChecksum)
	}

	return nil
}

// decodeFrom decodes the feed from the reader, validating length and checksum.
// It expects the format: [payloadLen(uint32)][checksum(uint32)][payload...].
func (f *Feed) decodeFrom(r io.Reader) (err error) {
	buf := buffer.Get()
	defer buffer.Put(buf)

	if err := f.validateFrom(r, buf); err != nil {
		return errors.Wrap(err, "validate payload")
	}

	payloadReader := bytes.NewReader(buf.B[feedHeaderSize:])
	if err := f.decodePayload(payloadReader); err != nil {
		return errors.Wrap(err, "decode payload")
	}

	return nil
}

// decodePayload decodes the core fields from the reader.
func (f *Feed) decodePayload(r io.Reader) error {
	f.Feed = &model.Feed{} // Ensure Feed is initialized.

	// Read ID.
	if err := binary.Read(r, binary.LittleEndian, &f.ID); err != nil {
		return errors.Wrap(err, "read id")
	}

	// Read time.
	var timestamp int64
	if err := binary.Read(r, binary.LittleEndian, &timestamp); err != nil {
		return errors.Wrap(err, "read time")
	}
	f.Time = time.Unix(0, timestamp).In(time.UTC)

	// Read labels.
	if err := f.decodeLabels(r); err != nil {
		return errors.Wrap(err, "decode labels")
	}

	// Read vectors.
	if err := f.decodeVectors(r); err != nil {
		return errors.Wrap(err, "decode vectors")
	}

	return nil
}

// decodeLabels reads the label data from the reader.
func (f *Feed) decodeLabels(r io.Reader) error {
	var labelCount uint32
	if err := binary.Read(r, binary.LittleEndian, &labelCount); err != nil {
		return errors.Wrap(err, "read labels count")
	}

	f.Labels = make(model.Labels, labelCount)
	for i := range labelCount {
		// Read key.
		key, err := binaryutil.ReadString(r)
		if err != nil {
			return errors.Wrapf(err, "read label key index %d", i)
		}

		// Read value.
		value, err := binaryutil.ReadString(r)
		if err != nil {
			return errors.Wrapf(err, "read label value index %d", i)
		}

		f.Labels[i] = model.Label{
			Key:   key,
			Value: value,
		}
	}

	return nil
}

// decodeVectors reads the vector data from the reader.
func (f *Feed) decodeVectors(r io.Reader) error {
	var vectorCount uint32
	if err := binary.Read(r, binary.LittleEndian, &vectorCount); err != nil {
		return errors.Wrap(err, "read vectors count")
	}
	if vectorCount == 0 {
		f.Vectors = nil // Ensure vectors is nil if count is 0

		return nil

	}
	f.Vectors = make([][]float32, vectorCount)

	var dimension uint32
	if err := binary.Read(r, binary.LittleEndian, &dimension); err != nil {
		return errors.Wrap(err, "read vector dimension")
	}

	// Pre-allocate the underlying float data contiguously for potentially better cache locality.
	totalFloats := uint64(vectorCount) * uint64(dimension)
	floatData := make([]float32, totalFloats)

	offset := 0
	for i := range vectorCount {
		f.Vectors[i] = floatData[offset : offset+int(dimension)] // Slice into the pre-allocated data
		if err := binary.Read(r, binary.LittleEndian, f.Vectors[i]); err != nil {
			return errors.Wrapf(err, "read vector data for vector %d", i)
		}
		offset += int(dimension)
	}

	return nil
}
