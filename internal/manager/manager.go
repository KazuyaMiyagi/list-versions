package manager

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// File is the unit of work handed to Manager.Extract. Path is the matched file
// or — for managers that implement DirScoped — the directory. Data holds the
// file's contents, pre-read once by the scanner and shared across every manager
// that claimed the same file; it is nil for DirScoped managers. Managers should
// obtain the bytes via readBytes so standalone Extract calls (with Data unset)
// still work.
type File struct {
	Path string
	Data []byte
}

// Manager classifies files during the single shared filesystem walk and pulls
// version Entries out of the ones it claims.
type Manager interface {
	Name() string
	// Match reports whether this manager wants the file at path. It must be a
	// pure predicate over the path/basename: the walk is shared across every
	// manager, so Match must not touch the filesystem.
	Match(path string) bool
	// Extract pulls Entries from a single matched File.
	Extract(file File) ([]entry.Entry, error)
}

// DirScoped is an optional Manager extension for managers that process a whole
// directory at once rather than file by file (a Terraform module spans several
// *.tf files that must be read together). When GroupByDir reports true, the
// scanner collapses the manager's matched files to their parent directories and
// calls Extract once per directory with File{Path: dir} (Data nil).
type DirScoped interface {
	Manager
	GroupByDir() bool
}

func dirScoped(m Manager) bool {
	ds, ok := m.(DirScoped)
	return ok && ds.GroupByDir()
}

// defaultPruneDirs are directory basenames the walk never descends into. It is
// read-only; per-run extras arrive through ScanAll's excludes argument, so the
// package keeps no shared mutable state.
var defaultPruneDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	".terraform":   true,
	"vendor":       true,
	".bundle":      true,
}

// pruneSet merges the read-only defaults with the caller's extra excludes into
// a fresh map, leaving defaultPruneDirs untouched.
func pruneSet(excludes []string) map[string]bool {
	out := make(map[string]bool, len(defaultPruneDirs)+len(excludes))
	for k := range defaultPruneDirs {
		out[k] = true
	}
	for _, e := range excludes {
		if e != "" {
			out[e] = true
		}
	}
	return out
}

// readBytes returns f.Data when the scanner has pre-read it, otherwise reads
// the file from disk. Managers call this instead of os.ReadFile/os.Open so a
// file claimed by several managers is read only once, while standalone Extract
// calls (e.g. tests building a File{Path: ...}) keep working.
func readBytes(f File) ([]byte, error) {
	if f.Data != nil {
		return stripBOM(f.Data), nil
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, err
	}
	return stripBOM(data), nil
}

// utf8BOM is the byte-order mark some editors (notably on Windows) prepend to
// UTF-8 files. It is not Unicode whitespace, so TrimSpace won't drop it; left
// in place it corrupts the first token (e.g. a version) and trips strict
// JSON/TOML decoders.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM removes a leading UTF-8 BOM so every manager sees clean bytes.
func stripBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

// walkFiles returns every non-pruned regular file under root in lexical order.
// Unreadable directories are skipped rather than aborting the walk, matching
// the inventory use case where one bad subtree must not blank out the report.
func walkFiles(root string, prune map[string]bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A failure on the root itself (missing path, unreadable dir) is
			// the caller's mistake and must surface; deeper errors only knock
			// out one subtree, so we skip those and keep the inventory going.
			if path == root {
				return err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && prune[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// safeExtract wraps Manager.Extract in a deferred recover so a buggy parser (or
// hostile input that trips one) converts to an error rather than tearing down
// the whole CLI. Inventory scans over many repos must not be destroyed by a
// single rogue file.
func safeExtract(m Manager, f File) (es []entry.Entry, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in manager: %v", r)
			es = nil
		}
	}()
	return m.Extract(f)
}

// ScanAll walks each root exactly once and dispatches every file to the
// managers that claim it, returning the entries grouped by root in input order.
// Callers flatten the groups to preserve user-specified root order in the final
// output.
//
// A single shared walk replaces the previous per-manager traversal, and each
// claimed file is read from disk only once even when several managers want it.
// Per-file failures (unreadable file, parse error surfaced by a manager) are
// reported to warnW and the scan continues; the inventory should survive one
// bad file. excludes adds directory basenames to the read-only default prune
// set for this run only.
//
// A root that cannot be accessed at all (missing path, unreadable directory)
// is warned to warnW and skipped — its slot in the returned slice is left
// empty so the remaining roots still produce output. When one or more roots
// were inaccessible, ScanAll returns a non-nil error after processing them all
// so the caller can emit what it has and still exit non-zero.
func ScanAll(roots []string, managers []Manager, excludes []string, warnW io.Writer) ([][]entry.Entry, error) {
	prune := pruneSet(excludes)
	out := make([][]entry.Entry, len(roots))
	var failed []string
	for i, root := range roots {
		entries, err := scanRoot(root, managers, prune, warnW)
		if err != nil {
			if warnW != nil {
				fmt.Fprintf(warnW, "list-versions: %s: %v\n", root, err)
			}
			failed = append(failed, root)
			continue
		}
		out[i] = entries
	}
	if len(failed) > 0 {
		return out, fmt.Errorf("could not scan %d path(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return out, nil
}

func scanRoot(root string, managers []Manager, prune map[string]bool, warnW io.Writer) ([]entry.Entry, error) {
	files, err := walkFiles(root, prune)
	if err != nil {
		return nil, err
	}

	warn := func(path, name string, err error) {
		if warnW != nil {
			fmt.Fprintf(warnW, "list-versions: %s (%s): %v\n", path, name, err)
		}
	}

	// Classify the walk results: file-scoped managers index by file path,
	// dir-scoped managers accumulate the set of directories they care about.
	fileClaims := map[string][]int{} // path -> indices of file-scoped managers
	var filePaths []string           // claimed paths, in walk order, deduped
	dirClaims := make([]map[string]bool, len(managers))

	for _, path := range files {
		for mi, m := range managers {
			if !m.Match(path) {
				continue
			}
			if dirScoped(m) {
				if dirClaims[mi] == nil {
					dirClaims[mi] = map[string]bool{}
				}
				dirClaims[mi][filepath.Dir(path)] = true
				continue
			}
			if _, seen := fileClaims[path]; !seen {
				filePaths = append(filePaths, path)
			}
			fileClaims[path] = append(fileClaims[path], mi)
		}
	}

	var entries []entry.Entry

	// File-scoped: read each claimed file once, hand the bytes to every manager
	// that claimed it.
	for _, path := range filePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			for _, mi := range fileClaims[path] {
				warn(path, managers[mi].Name(), err)
			}
			continue
		}
		for _, mi := range fileClaims[path] {
			es, err := safeExtract(managers[mi], File{Path: path, Data: data})
			if err != nil {
				warn(path, managers[mi].Name(), err)
				continue
			}
			entries = append(entries, es...)
		}
	}

	// Dir-scoped: one Extract per claimed directory, in lexical order.
	for mi := range managers {
		if dirClaims[mi] == nil {
			continue
		}
		dirs := make([]string, 0, len(dirClaims[mi]))
		for d := range dirClaims[mi] {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			es, err := safeExtract(managers[mi], File{Path: d})
			if err != nil {
				warn(d, managers[mi].Name(), err)
				continue
			}
			entries = append(entries, es...)
		}
	}

	return entries, nil
}
