package chunk

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vandeefeng/zenfeed/pkg/model"
)

// --- Benchmark Setup ---

const (
	benchmarkFeedCount = 10000 // Number of feeds for benchmark setup
	benchmarkBatchSize = 100   // Batch size for append benchmark
)

var (
	benchmarkFeeds    []*Feed
	benchmarkOffsets  []uint64 // Store offsets for read benchmark
	benchmarkTempPath string
)

// setupBenchmarkFile creates a temporary file and populates it with benchmarkFeeds.
// It returns the path and a cleanup function.
func setupBenchmarkFile(b *testing.B, readonly bool) (File, func()) {
	b.Helper()

	// Create temp file path only once
	if benchmarkTempPath == "" {
		dir, err := os.MkdirTemp("", "chunk-benchmark")
		if err != nil {
			b.Fatalf("Failed to create temp dir: %v", err)
		}
		benchmarkTempPath = filepath.Join(dir, "benchmark.chunk")
	}
	cleanup := func() {
		os.RemoveAll(filepath.Dir(benchmarkTempPath))
		benchmarkTempPath = "" // Reset path for next potential setup
		benchmarkFeeds = nil   // Clear feeds
		benchmarkOffsets = nil // Clear offsets
	}

	// Generate feeds only once per setup phase if needed
	if len(benchmarkFeeds) == 0 {
		benchmarkFeeds = generateBenchmarkFeeds(benchmarkFeedCount)
		benchmarkOffsets = make([]uint64, 0, benchmarkFeedCount)
	}

	// Create and populate the file in read-write mode first
	rwConfig := &Config{Path: benchmarkTempPath}
	rwFile, err := new("benchmark-setup", rwConfig, Dependencies{})
	if err != nil {
		cleanup()
		b.Fatalf("Failed to create benchmark file for setup: %v", err)
	}

	currentOffsetCount := int(rwFile.Count(context.Background()))
	if currentOffsetCount < benchmarkFeedCount { // Only append if not already populated
		appendCount := 0
		onSuccess := func(feed *Feed, offset uint64) error {
			// Collect offsets only during the initial population
			if len(benchmarkOffsets) < benchmarkFeedCount {
				benchmarkOffsets = append(benchmarkOffsets, offset)
			}
			appendCount++
			return nil
		}
		for i := currentOffsetCount; i < benchmarkFeedCount; i += benchmarkBatchSize {
			end := i + benchmarkBatchSize
			if end > benchmarkFeedCount {
				end = benchmarkFeedCount
			}
			if err := rwFile.Append(context.Background(), benchmarkFeeds[i:end], onSuccess); err != nil {
				rwFile.Close()
				cleanup()
				b.Fatalf("Failed to append feeds during setup: %v", err)
			}
		}
	}
	// Close the read-write file before potentially reopening as readonly
	if err := rwFile.Close(); err != nil {
		cleanup()
		b.Fatalf("Failed to close rw file during setup: %v", err)
	}

	// Reopen file with the desired mode for the benchmark
	config := &Config{
		Path:            benchmarkTempPath,
		ReadonlyAtFirst: readonly,
	}
	f, err := new("benchmark", config, Dependencies{})
	if err != nil {
		cleanup()
		b.Fatalf("Failed to open benchmark file in target mode: %v", err)
	}

	if readonly {
		// For read benchmarks, ensure mmap is active if file was just created/populated
		if err := f.EnsureReadonly(context.Background()); err != nil {
			f.Close()
			cleanup()
			b.Fatalf("Failed to ensure readonly mode: %v", err)
		}
	}

	return f, cleanup
}

func generateBenchmarkFeeds(count int) []*Feed {
	feeds := make([]*Feed, count)
	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // Use a fixed seed for reproducibility if needed
	// Pre-generate some random characters for building large strings efficiently.
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	letterRunes := []rune(letters)
	randString := func(n int) string {
		sb := strings.Builder{}
		sb.Grow(n)
		for i := 0; i < n; i++ {
			sb.WriteRune(letterRunes[rng.Intn(len(letterRunes))])
		}
		return sb.String()
	}

	minLabelSize := 8 * 1024  // 8KB
	maxLabelSize := 15 * 1024 // 15KB

	for i := range count {
		// Generate large label content size.
		largeLabelSize := minLabelSize + rng.Intn(maxLabelSize-minLabelSize+1)
		// Estimate the overhead of other labels and structure (key names, length prefixes etc.).
		// This is a rough estimation, adjust if needed.
		otherLabelsOverhead := 100
		largeContentSize := largeLabelSize - otherLabelsOverhead
		if largeContentSize < 0 {
			largeContentSize = 0
		}

		feeds[i] = &Feed{
			Feed: &model.Feed{
				ID: uint64(i + 1),
				Labels: model.Labels{
					model.Label{Key: "type", Value: fmt.Sprintf("type_%d", rng.Intn(10))},
					model.Label{Key: "source", Value: fmt.Sprintf("source_%d", rng.Intn(5))},
					model.Label{Key: "large_content", Value: randString(largeContentSize)}, // Add large label
				},
				Time: time.Now().Add(-time.Duration(rng.Intn(3600*24*30)) * time.Second), // Random time within the last 30 days
			},
			Vectors: [][]float32{
				generateFloat32Vector(rng, 1024), // Example dimension
				generateFloat32Vector(rng, 1024),
			},
		}
	}
	return feeds
}

func generateFloat32Vector(rng *rand.Rand, dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()
	}
	return vec
}

// --- Benchmarks ---

func BenchmarkAppend(b *testing.B) {
	// Setup: Start with an empty file for appending.
	// Note: setupBenchmarkFile(b, false) creates the file but doesn't populate it fully here.
	// We need a fresh file for append benchmark.
	dir, err := os.MkdirTemp("", "chunk-append-benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	path := filepath.Join(dir, "append_benchmark.chunk")
	cleanup := func() {
		os.RemoveAll(dir)
	}
	defer cleanup()

	config := &Config{Path: path}
	f, err := new("benchmark-append", config, Dependencies{})
	if err != nil {
		b.Fatalf("Failed to create benchmark file for append: %v", err)
	}
	defer f.Close()

	feedsToAppend := generateBenchmarkFeeds(benchmarkBatchSize) // Generate a batch

	b.ResetTimer()
	b.ReportAllocs()
	// Measure appending batches of feeds.
	for i := 0; i < b.N; i++ {
		// Simulate appending new batches. In a real scenario, feeds would differ.
		// For benchmark consistency, we reuse the same batch data.
		err := f.Append(context.Background(), feedsToAppend, nil) // onSuccess is nil for performance
		if err != nil {
			b.Fatalf("Append failed during benchmark: %v", err)
		}
	}
	b.StopTimer() // Stop timer before potential cleanup/close overhead
}

func BenchmarkRead(b *testing.B) {
	// Setup: Populate a file and make it readonly (mmap).
	f, cleanup := setupBenchmarkFile(b, true)
	defer cleanup()

	if len(benchmarkOffsets) == 0 {
		b.Fatal("Benchmark setup failed: no offsets generated.")
	}

	// Pre-select random offsets to read
	rng := rand.New(rand.NewSource(42)) // Use a fixed seed for reproducibility
	readIndices := make([]int, b.N)
	for i := 0; i < b.N; i++ {
		readIndices[i] = rng.Intn(len(benchmarkOffsets))
	}

	b.ResetTimer()
	b.ReportAllocs()
	// Measure reading feeds at random valid offsets using mmap.
	for i := 0; i < b.N; i++ {
		offset := benchmarkOffsets[readIndices[i]]
		feed, err := f.Read(context.Background(), offset)
		if err != nil {
			b.Fatalf("Read failed during benchmark at offset %d: %v", offset, err)
		}
		// Prevent compiler optimization by using the result slightly
		if feed == nil {
			b.Fatal("Read returned nil feed")
		}
	}
	b.StopTimer()
}

func BenchmarkRange(b *testing.B) {
	// Setup: Populate a file and make it readonly (mmap).
	f, cleanup := setupBenchmarkFile(b, false)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()
	// Measure ranging over all feeds using mmap.
	for i := 0; i < b.N; i++ {
		count := 0
		err := f.Range(context.Background(), func(feed *Feed, offset uint64) (err error) {
			// Minimal operation inside the iterator
			count++
			if feed == nil { // Basic check
				return fmt.Errorf("nil feed encountered at offset %d", offset)
			}
			return nil
		})
		if err != nil {
			b.Fatalf("Range failed during benchmark: %v", err)
		}
		// Optionally verify count, though it adds overhead to the benchmark itself
		// if uint32(count) != f.Count(context.Background()) {
		// 	b.Fatalf("Range count mismatch: expected %d, got %d", f.Count(context.Background()), count)
		// }
	}
	b.StopTimer()
}
