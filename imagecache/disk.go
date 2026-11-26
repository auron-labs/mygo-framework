package imagecache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	cacheFileNameBytes   = 32 // SHA-256.
	cacheFileHexChars    = cacheFileNameBytes * 2
	tempFilePattern      = "imagecache-*.tmp"
	maxDiskMetadataBytes = 16 << 10
)

// diskMetadata is the JSON sidecar stored beside a body. It deliberately does
// not contain the URL; the file name is the URL's SHA-256 digest.
type diskMetadata struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"lastModified,omitempty"`
	FreshUntil   time.Time `json:"freshUntil"`
}

func diskPaths(dir, rawURL string) (body, meta string) {
	sum := sha256.Sum256([]byte(rawURL))
	name := hex.EncodeToString(sum[:])
	return filepath.Join(dir, name), filepath.Join(dir, name+".json")
}

// readDiskFiles reads a body only after bounding its size. The filesystem is
// untrusted input: a user or another process may have replaced cache files.
func readDiskFiles(dir, rawURL string, maxBytes int64) ([]byte, diskMetadata, bool) {
	bodyPath, metaPath := diskPaths(dir, rawURL)
	info, err := os.Stat(bodyPath)
	if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > maxBytes {
		return nil, diskMetadata{}, false
	}
	data, err := readFileLimited(bodyPath, maxBytes)
	if err != nil || len(data) == 0 {
		return nil, diskMetadata{}, false
	}
	meta, _ := readDiskMetadata(metaPath)
	return data, meta, true
}

func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path) // #nosec G304 -- path is a SHA-256 cache file inside the configured cache directory.
	if err != nil {
		return nil, fmt.Errorf("open disk cache body: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read disk cache body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("imagecache: disk body exceeds size limit")
	}
	return data, nil
}

func readDiskMetadata(path string) (diskMetadata, bool) {
	raw, err := readFileLimited(path, maxDiskMetadataBytes)
	if err != nil {
		return diskMetadata{}, false
	}
	var meta diskMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return diskMetadata{}, false
	}
	return meta, true
}

// writeDiskFiles writes body and sidecar atomically. Both files are private;
// temporary files are removed on every failure path.
func writeDiskFiles(dir, rawURL string, data []byte, meta diskMetadata) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create image cache directory: %w", err)
	}
	bodyPath, metaPath := diskPaths(dir, rawURL)
	if err := writeFileAtomic(bodyPath, data); err != nil {
		return fmt.Errorf("write disk cache body: %w", err)
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		_ = os.Remove(bodyPath)
		return fmt.Errorf("encode disk cache metadata: %w", err)
	}
	if err := writeFileAtomic(metaPath, raw); err != nil {
		_ = os.Remove(bodyPath)
		return fmt.Errorf("write disk cache metadata: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), tempFilePattern)
	if err != nil {
		return fmt.Errorf("create temporary cache file: %w", err)
	}
	tmpName := tmp.Name()
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary cache file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace disk cache file: %w", err)
	}
	complete = true
	return nil
}

func writeDiskMetadataIfCurrent(cache *Cache, rawURL string, token diskToken, meta diskMetadata) {
	cache.diskMu.Lock()
	defer cache.diskMu.Unlock()
	if !cache.diskTokenCurrent(rawURL, token) {
		return
	}
	_, metaPath := diskPaths(cache.dir, rawURL)
	if raw, err := json.Marshal(meta); err == nil {
		_ = writeFileAtomic(metaPath, raw)
	}
}

func removeDiskFiles(dir, rawURL string) error {
	bodyPath, metaPath := diskPaths(dir, rawURL)
	return errors.Join(removeIfExists(bodyPath), removeIfExists(metaPath))
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove disk cache file: %w", err)
	}
	return nil
}

func isCacheBodyName(name string) bool {
	return len(name) == cacheFileHexChars && isLowerHex(name)
}

func isCacheMetaName(name string) bool {
	base, ok := strings.CutSuffix(name, ".json")
	return ok && len(base) == cacheFileHexChars && isLowerHex(base)
}

func isCacheTempName(name string) bool {
	return strings.HasPrefix(name, "imagecache-") && strings.HasSuffix(name, ".tmp")
}

func isLowerHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// clearDiskFiles removes only files whose names are owned by this cache. It is
// intentionally not os.RemoveAll: applications receive a dedicated directory,
// but a foreign file placed there by mistake is not cache data to delete.
func clearDiskFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read image cache directory: %w", err)
	}
	var result error
	for _, item := range entries {
		if item.IsDir() || (!isCacheBodyName(item.Name()) && !isCacheMetaName(item.Name()) && !isCacheTempName(item.Name())) {
			continue
		}
		result = errors.Join(result, removeIfExists(filepath.Join(dir, item.Name())))
	}
	return result
}

type diskFileInfo struct {
	path    string
	size    int64
	modTime time.Time
}

// pruneDiskFiles removes cache temporaries, corrupt sidecars, impossible
// bodies, orphan sidecars, and then oldest bodies until cache-owned files fit
// limit. Foreign file names are neither counted nor removed.
func pruneDiskFiles(dir string, limit int64) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read image cache directory: %w", err)
	}

	bodies := map[string]diskFileInfo{}
	sidecars := map[string]diskFileInfo{}
	var result error
	for _, item := range entries {
		if item.IsDir() {
			continue
		}
		name := item.Name()
		path := filepath.Join(dir, name)
		if isCacheTempName(name) {
			result = errors.Join(result, removeIfExists(path))
			continue
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		file := diskFileInfo{path: path, size: info.Size(), modTime: info.ModTime()}
		if isCacheBodyName(name) {
			bodies[name] = file
			continue
		}
		if isCacheMetaName(name) {
			base := strings.TrimSuffix(name, ".json")
			if _, ok := readDiskMetadata(path); !ok {
				result = errors.Join(result, removeIfExists(path))
				continue
			}
			sidecars[base] = file
		}
	}

	var total int64
	ordered := make([]diskFileInfo, 0, len(bodies))
	for name, body := range bodies {
		if body.size == 0 || body.size > maxImageBytes {
			result = errors.Join(result, removeIfExists(body.path))
			if sidecar, ok := sidecars[name]; ok {
				result = errors.Join(result, removeIfExists(sidecar.path))
				delete(sidecars, name)
			}
			delete(bodies, name)
			continue
		}
		total += body.size
		if sidecar, ok := sidecars[name]; ok {
			total += sidecar.size
		}
		ordered = append(ordered, body)
	}
	for base, sidecar := range sidecars {
		if _, ok := bodies[base]; !ok {
			result = errors.Join(result, removeIfExists(sidecar.path))
		}
	}

	slices.SortFunc(ordered, func(a, b diskFileInfo) int { return a.modTime.Compare(b.modTime) })
	for _, body := range ordered {
		if total <= limit {
			break
		}
		if err := removeIfExists(body.path); err != nil {
			result = errors.Join(result, err)
			continue
		}
		total -= body.size
		base := filepath.Base(body.path)
		if sidecar, ok := sidecars[base]; ok {
			if err := removeIfExists(sidecar.path); err != nil {
				result = errors.Join(result, err)
			} else {
				total -= sidecar.size
			}
		}
	}
	return result
}

// storeIfCurrent serializes with Forget and Clear. The entry and token checks
// happen while diskMu is held: if a privacy deletion won the mutex first the
// write is skipped, and if the write won first the deletion removes it after.
func (c *Cache) storeIfCurrent(e *entry, result fetched) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	c.mu.Lock()
	current := c.entries[e.key] == e && !e.forgotten && result.token.clear == c.clearEpoch && result.token.url == c.urlEpochs[e.key.url]
	c.mu.Unlock()
	if !current || len(result.data) == 0 || int64(len(result.data)) > maxImageBytes {
		return
	}
	if err := writeDiskFiles(c.dir, e.key.url, result.data, result.metadata); err != nil {
		return
	}
	_ = pruneDiskFiles(c.dir, c.diskByteLimit)
}

func (c *Cache) removeDiskFiles(result fetched) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if !c.diskTokenCurrent(result.url, result.token) {
		return
	}
	_ = removeDiskFiles(c.dir, result.url)
}

func (c *Cache) removeDiskFilesIfCurrent(rawURL string, token diskToken) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if !c.diskTokenCurrent(rawURL, token) {
		return
	}
	_ = removeDiskFiles(c.dir, rawURL)
}
