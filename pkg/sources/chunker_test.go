package sources

import (
	"bytes"
	"io"
	"strings"
	"testing"

	diskbufferreader "github.com/bill-rich/disk-buffer-reader"
	"github.com/stretchr/testify/assert"

	"github.com/trufflesecurity/trufflehog/v3/pkg/context"
)

func TestChunker(t *testing.T) {
	byteBuffer := bytes.NewBuffer(make([]byte, ChunkSize*9))
	reReader, err := diskbufferreader.New(byteBuffer)
	if err != nil {
		t.Fatal(err)
	}
	defer reReader.Close()

	baseChunks := make([]*Chunk, 0, 9)

	chunkData, _ := io.ReadAll(reReader)
	originalChunk := &Chunk{
		Data: chunkData,
	}

	for chunk := range Chunker(originalChunk) {
		baseChunks = append(baseChunks, chunk)
	}

	_ = reReader.Reset()

	testChunks := make([]*Chunk, 0, 9)

	testData, _ := io.ReadAll(reReader)
	testOriginalChunk := &Chunk{
		Data: testData,
	}

	for chunk := range Chunker(testOriginalChunk) {
		testChunks = append(testChunks, chunk)
	}

	if len(testChunks) != len(baseChunks) {
		t.Errorf("Wrong number of chunks received. Got %d, expected: %d.", len(testChunks), len(baseChunks))
	}

	for i, baseChunk := range baseChunks {
		if !bytes.Equal(baseChunk.Data, testChunks[i].Data) {
			t.Errorf("Chunk %d did not match expected. Got: %d bytes, expected: %d bytes", i+1, len(testChunks[i].Data), len(baseChunk.Data))
		}
	}
}

func BenchmarkChunker(b *testing.B) {
	data := bytes.Repeat([]byte("a"), ChunkSize*100)
	chunk := &Chunk{
		Data: data,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range Chunker(chunk) {
		}
	}
}

func TestNewChunkedReader(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		chunkSize  int
		peekSize   int
		wantChunks []string
		wantErr    bool
	}{
		{
			name:       "Smaller data than default chunkSize and peekSize",
			input:      "example input",
			chunkSize:  ChunkSize,
			peekSize:   PeekSize,
			wantChunks: []string{"example input"},
			wantErr:    false,
		},
		{
			name:       "Reader with no data",
			input:      "",
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{},
			wantErr:    false,
		},
		{
			name:       "Smaller data than chunkSize and peekSize",
			input:      "small data",
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{"small data"},
			wantErr:    false,
		},
		{
			name:       "Equal to chunkSize",
			input:      strings.Repeat("a", 1024),
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{strings.Repeat("a", 1024)},
			wantErr:    false,
		},
		{
			name:       "Equal to chunkSize + peekSize",
			input:      strings.Repeat("a", 1536),
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{strings.Repeat("a", 1536), strings.Repeat("a", 512)},
			wantErr:    false,
		},
		{
			name:       "EOF during peeking",
			input:      strings.Repeat("a", 1300),
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{strings.Repeat("a", 1300), strings.Repeat("a", 276)},
			wantErr:    false,
		},
		{
			name:       "EOF during reading",
			input:      strings.Repeat("a", 512),
			chunkSize:  1024,
			peekSize:   512,
			wantChunks: []string{strings.Repeat("a", 512)},
			wantErr:    false,
		},
		{
			name:       "Equal to totalSize",
			input:      strings.Repeat("a", 2048),
			chunkSize:  1024,
			peekSize:   1024,
			wantChunks: []string{strings.Repeat("a", 2048), strings.Repeat("a", 1024)},
			wantErr:    false,
		},
		{
			name:       "Larger than totalSize",
			input:      strings.Repeat("a", 4096),
			chunkSize:  1024,
			peekSize:   1024,
			wantChunks: []string{strings.Repeat("a", 2048), strings.Repeat("a", 2048), strings.Repeat("a", 2048), strings.Repeat("a", 1024)},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readerFunc := NewChunkReader(WithChunkSize(tt.chunkSize), WithPeekSize(tt.peekSize))
			reader := strings.NewReader(tt.input)
			ctx := context.Background()
			chunkResChan := readerFunc(ctx, reader)

			var err error
			chunks := make([]string, 0)
			for data := range chunkResChan {
				chunks = append(chunks, string(data.Bytes()))
				err = data.Error()
			}

			assert.Equal(t, tt.wantChunks, chunks, "Chunks do not match")
			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func BenchmarkChunkReader(b *testing.B) {
	var bigChunk = make([]byte, 1<<24) // 16MB

	reader := bytes.NewReader(bigChunk)
	chunkReader := NewChunkReader(WithChunkSize(ChunkSize), WithPeekSize(PeekSize))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StartTimer()
		chunkResChan := chunkReader(context.Background(), reader)

		// Drain the channel.
		for range chunkResChan {
		}

		b.StopTimer()
		_, err := reader.Seek(0, 0)
		assert.Nil(b, err)
	}
}
