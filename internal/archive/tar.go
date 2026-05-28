package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/Origin173/SnapCraft/internal/config"
)

// Result holds archive creation statistics.
type Result struct {
	Path       string
	FileCount  int64
	TotalBytes int64
}

// Create builds a tar archive from source paths.
func Create(cfg *config.Config, destPath string, sources []string) (*Result, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return nil, err
	}

	f, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var writer io.Writer = f
	var closer io.Closer

	switch cfg.Backup.Compression {
	case config.CompressionGzip:
		gw := gzip.NewWriter(f)
		writer = gw
		closer = gw
	case config.CompressionZstd:
		zw, err := zstd.NewWriter(f)
		if err != nil {
			return nil, err
		}
		writer = zw
		closer = zw
	case config.CompressionNone:
		// raw tar
	default:
		return nil, fmt.Errorf("unsupported compression: %s", cfg.Backup.Compression)
	}

	tw := tar.NewWriter(writer)
	result := &Result{Path: destPath}

	for _, src := range sources {
		src = filepath.Clean(src)
		info, err := os.Stat(src)
		if err != nil {
			tw.Close()
			if closer != nil {
				closer.Close()
			}
			return nil, err
		}
		base := filepath.Base(src)
		if info.IsDir() {
			if err := addDir(tw, src, base, result); err != nil {
				tw.Close()
				if closer != nil {
					closer.Close()
				}
				return nil, err
			}
		} else {
			if err := addFile(tw, src, base, info, result); err != nil {
				tw.Close()
				if closer != nil {
					closer.Close()
				}
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			return nil, err
		}
	}

	st, err := os.Stat(destPath)
	if err != nil {
		return nil, err
	}
	result.TotalBytes = st.Size()
	return result, nil
}

func addDir(tw *tar.Writer, srcDir, prefix string, result *Result) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == srcDir {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))
		if info.IsDir() {
			hdr := &tar.Header{Name: name + "/", Mode: int64(info.Mode()), ModTime: info.ModTime(), Typeflag: tar.TypeDir}
			return tw.WriteHeader(hdr)
		}
		return addFile(tw, path, name, info, result)
	})
}

func addFile(tw *tar.Writer, path, name string, info os.FileInfo, result *Result) error {
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(tw, f)
	result.FileCount++
	result.TotalBytes += n
	return err
}

// ArchiveExtension returns the file extension for the configured compression.
func ArchiveExtension(cfg *config.Config) string {
	switch cfg.Backup.Compression {
	case config.CompressionZstd:
		return ".tar.zst"
	case config.CompressionGzip:
		return ".tar.gz"
	default:
		return ".tar"
	}
}

// ResolveSources returns paths to include in archive backup.
func ResolveSources(cfg *config.Config) []string {
	if len(cfg.Backup.Archive.IncludePaths) > 0 {
		return cfg.Backup.Archive.IncludePaths
	}
	return []string{cfg.Server.WorldPath}
}

// Extract unpacks an archive to destDir.
func Extract(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	switch {
	case strings.HasSuffix(archivePath, ".tar.zst"):
		zr, err := zstd.NewReader(f)
		if err != nil {
			return err
		}
		defer zr.Close()
		reader = zr
	case strings.HasSuffix(archivePath, ".tar.gz"):
		gr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gr.Close()
		reader = gr
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) &&
			filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid tar path: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}
