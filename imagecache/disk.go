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
	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode disk cache metadata: %w", err)
	}
	return writeDiskFilesRaw(dir, rawURL, data, raw)
}

func writeDiskFilesRaw(dir, rawURL string, data, rawMeta []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create image cache directory: %w", err)
	}
	bodyPath, metaPath := diskPaths(dir, rawURL)
	if err := writeFileAtomic(bodyPath, data); err != nil {
		return fmt.Errorf("write disk cache body: %w", err)
	}
	if err := writeFileAtomic(metaPath, rawMeta); err != nil {
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
	index := cache.ensureDiskIndexLocked()
	raw, err := json.Marshal(meta)
	if err != nil {
		return
	}
	_, metaPath := diskPaths(cache.dir, rawURL)
	if err := writeFileAtomic(metaPath, raw); err != nil {
		return
	}
	cache.noteDiskMetadataWriteLocked(rawURL, int64(len(raw)))
	if index.total > cache.diskByteLimit {
		_ = pruneDiskIndex(cache.dir, index, cache.diskByteLimit)
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

type diskIndexEntry struct {
	bodySize int64
	metaSize int64
	modTime  time.Time
}

func (entry diskIndexEntry) size() int64 {
	return entry.bodySize + entry.metaSize
}

// diskIndex is the cache's in-memory view of cache-owned disk files. It is
// scanned at most once per Cache instance and afterwards maintained by the
// serialized writers, so storing one image never scans or parses the whole
// directory again.
type diskIndex struct {
	scanned bool
	total   int64
	files   map[string]diskIndexEntry
}

// scanDiskIndex performs the one expensive cache-directory pass. It removes
// interrupted temporaries and malformed or orphan cache artifacts, validates
// sidecars once, and records sizes and use times for later routine pruning.
func scanDiskIndex(dir string) (diskIndex, error) {
	index := diskIndex{scanned: true, files: map[string]diskIndexEntry{}}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return index, nil
	}
	if err != nil {
		index.scanned = false
		return index, fmt.Errorf("read image cache directory: %w", err)
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
			if file.size == 0 || file.size > maxImageBytes {
				result = errors.Join(result, removeIfExists(path))
				continue
			}
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

	for name, body := range bodies {
		entry := diskIndexEntry{bodySize: body.size, modTime: body.modTime}
		index.total += body.size
		if sidecar, ok := sidecars[name]; ok {
			entry.metaSize = sidecar.size
			index.total += sidecar.size
			if sidecar.modTime.After(entry.modTime) {
				entry.modTime = sidecar.modTime
			}
		}
		index.files[name] = entry
	}
	for base, sidecar := range sidecars {
		if _, ok := bodies[base]; !ok {
			result = errors.Join(result, removeIfExists(sidecar.path))
		}
	}
	return index, result
}

type diskPruneCandidate struct {
	name  string
	entry diskIndexEntry
}

// pruneDiskIndex uses only the in-memory index. Routine pruning therefore
// stats no files and parses no sidecars; it merely unlinks oldest entries
// until the tracked total fits the configured limit.
func pruneDiskIndex(dir string, index *diskIndex, limit int64) error {
	if index.total <= limit {
		return nil
	}
	ordered := make([]diskPruneCandidate, 0, len(index.files))
	for name, entry := range index.files {
		ordered = append(ordered, diskPruneCandidate{name: name, entry: entry})
	}
	slices.SortFunc(ordered, func(a, b diskPruneCandidate) int {
		if byTime := a.entry.modTime.Compare(b.entry.modTime); byTime != 0 {
			return byTime
		}
		return strings.Compare(a.name, b.name)
	})

	var result error
	for _, candidate := range ordered {
		if index.total <= limit {
			break
		}
		bodyPath := filepath.Join(dir, candidate.name)
		metaPath := filepath.Join(dir, candidate.name+".json")
		if err := errors.Join(removeIfExists(bodyPath), removeIfExists(metaPath)); err != nil {
			result = errors.Join(result, err)
			continue
		}
		index.total -= candidate.entry.size()
		delete(index.files, candidate.name)
	}
	return result
}

// pruneDiskFiles removes cache temporaries, corrupt sidecars, impossible
// bodies, orphan sidecars, and then oldest bodies until cache-owned files fit
// limit. Foreign file names are neither counted nor removed. It is kept as a
// standalone maintenance operation for tests; Cache uses its one-scan index
// rather than calling this after every write.
func pruneDiskFiles(dir string, limit int64) error {
	index, scanErr := scanDiskIndex(dir)
	return errors.Join(scanErr, pruneDiskIndex(dir, &index, limit))
}

func (c *Cache) ensureDiskIndexLocked() *diskIndex {
	if !c.diskIndex.scanned {
		index, _ := scanDiskIndex(c.dir)
		c.diskIndex = index
	}
	if c.diskIndex.files == nil {
		c.diskIndex.scanned = true
		c.diskIndex.files = map[string]diskIndexEntry{}
	}
	return &c.diskIndex
}

func diskIndexKey(dir, rawURL string) string {
	bodyPath, _ := diskPaths(dir, rawURL)
	return filepath.Base(bodyPath)
}

func (c *Cache) noteDiskWriteLocked(rawURL string, bodySize, metaSize int64) {
	index := c.ensureDiskIndexLocked()
	key := diskIndexKey(c.dir, rawURL)
	old := index.files[key]
	index.total += bodySize + metaSize - old.size()
	index.files[key] = diskIndexEntry{bodySize: bodySize, metaSize: metaSize, modTime: c.now()}
}

func (c *Cache) noteDiskMetadataWriteLocked(rawURL string, metaSize int64) {
	index := c.ensureDiskIndexLocked()
	key := diskIndexKey(c.dir, rawURL)
	entry := index.files[key]
	index.total += metaSize - entry.metaSize
	entry.metaSize = metaSize
	entry.modTime = c.now()
	index.files[key] = entry
}

func (c *Cache) noteDiskRemoveLocked(rawURL string) {
	index := c.ensureDiskIndexLocked()
	key := diskIndexKey(c.dir, rawURL)
	if entry, ok := index.files[key]; ok {
		index.total -= entry.size()
		delete(index.files, key)
	}
}

// touchDiskFiles records use of a disk hit. Recency is represented in file
// mtimes as well as in the in-memory index, so a warm cache scanned by the
// next instance still knows which long-lived avatars are used often.
func (c *Cache) touchDiskFiles(rawURL string, token diskToken) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if !c.diskTokenCurrent(rawURL, token) {
		return
	}
	now := c.now()
	bodyPath, metaPath := diskPaths(c.dir, rawURL)
	if err := os.Chtimes(bodyPath, now, now); err != nil {
		return
	}
	_ = os.Chtimes(metaPath, now, now)
	if c.diskIndex.scanned {
		key := diskIndexKey(c.dir, rawURL)
		if entry, ok := c.diskIndex.files[key]; ok {
			entry.modTime = now
			c.diskIndex.files[key] = entry
		}
	}
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
	rawMeta, err := json.Marshal(result.metadata)
	if err != nil {
		return
	}
	index := c.ensureDiskIndexLocked()
	if err := writeDiskFilesRaw(c.dir, e.key.url, result.data, rawMeta); err != nil {
		return
	}
	c.noteDiskWriteLocked(e.key.url, int64(len(result.data)), int64(len(rawMeta)))
	if index.total > c.diskByteLimit {
		_ = pruneDiskIndex(c.dir, index, c.diskByteLimit)
	}
}

func (c *Cache) removeDiskFiles(result fetched) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if !c.diskTokenCurrent(result.url, result.token) {
		return
	}
	if err := removeDiskFiles(c.dir, result.url); err == nil {
		c.noteDiskRemoveLocked(result.url)
	}
}

func (c *Cache) removeDiskFilesIfCurrent(rawURL string, token diskToken) {
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if !c.diskTokenCurrent(rawURL, token) {
		return
	}
	if err := removeDiskFiles(c.dir, rawURL); err == nil {
		c.noteDiskRemoveLocked(rawURL)
	}
}
