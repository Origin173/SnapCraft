package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

// Hasher computes content hashes.
type Hasher struct {
	method string
}

func NewHasher(method string) *Hasher {
	return &Hasher{method: method}
}

func (h *Hasher) Sum(r io.Reader) (string, error) {
	switch h.method {
	case "blake3":
		hasher := blake3.New()
		if _, err := io.Copy(hasher, r); err != nil {
			return "", err
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	default:
		hasher := sha256.New()
		if _, err := io.Copy(hasher, r); err != nil {
			return "", err
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	}
}

func (h *Hasher) SumFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	sum, err := h.Sum(f)
	return sum, st.Size(), err
}

func objectPath(baseDir, hash string) string {
	if len(hash) < 4 {
		return fmt.Sprintf("%s/%s", baseDir, hash)
	}
	return fmt.Sprintf("%s/%s/%s/%s", baseDir, hash[0:2], hash[2:4], hash)
}
