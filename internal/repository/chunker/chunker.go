package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

// Chunk represents a content-defined chunk.
type Chunk struct {
	Offset int64
	Size   int64
	Hash   string
	Data   []byte
}

// Config controls CDC behavior.
type Config struct {
	MinSize     int
	AvgSize     int
	MaxSize     int
	MinFileSize int64
}

// SplitFile reads a file and returns CDC chunks with hashes.
func SplitFile(path string, cfg Config, hashMethod string) ([]Chunk, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() < cfg.MinFileSize {
		return nil, nil
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return SplitBytes(data, cfg, hashMethod), nil
}

// SplitBytes performs content-defined chunking using a rolling hash.
func SplitBytes(data []byte, cfg Config, hashMethod string) []Chunk {
	if len(data) == 0 {
		return nil
	}
	minSize := cfg.MinSize
	if minSize <= 0 {
		minSize = 65536
	}
	avgSize := cfg.AvgSize
	if avgSize <= 0 {
		avgSize = 1048576
	}
	maxSize := cfg.MaxSize
	if maxSize <= 0 {
		maxSize = 4194304
	}

	var chunks []Chunk
	offset := 0
	for offset < len(data) {
		end := offset + minSize
		if end > len(data) {
			end = len(data)
		}
		if end < len(data) {
			scanLimit := offset + maxSize
			if scanLimit > len(data) {
				scanLimit = len(data)
			}
			cut := findCutPoint(data[offset:scanLimit], avgSize, minSize)
			end = offset + cut
		}
		part := data[offset:end]
		chunks = append(chunks, Chunk{
			Offset: int64(offset),
			Size:   int64(len(part)),
			Hash:   hashBytes(part, hashMethod),
			Data:   append([]byte(nil), part...),
		})
		offset = end
	}
	return chunks
}

func findCutPoint(window []byte, avgSize, minSize int) int {
	if len(window) <= minSize {
		return len(window)
	}
	var h uint32
	const mask uint32 = (1 << 13) - 1
	for i := minSize; i < len(window); i++ {
		h = (h << 1) + uint32(window[i])
		if (h & mask) == 0 {
			return i + 1
		}
		if i >= avgSize*2 {
			return i + 1
		}
	}
	return len(window)
}

func hashBytes(data []byte, method string) string {
	switch method {
	case "blake3":
		h := blake3.New()
		_, _ = h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	default:
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:])
	}
}
