package manager

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// UvToml reads `required-version` from uv.toml, the standalone form of the
// `[tool.uv]` table that PyprojectToml reads.
type UvToml struct{}

func (UvToml) Name() string { return "uv-toml" }

func (UvToml) Match(path string) bool { return filepath.Base(path) == "uv.toml" }

func (UvToml) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc struct {
		RequiredVersion string `toml:"required-version"`
	}
	if _, err := toml.Decode(string(data), &doc); err != nil {
		return nil, err
	}
	if doc.RequiredVersion == "" {
		return nil, nil
	}
	return []entry.Entry{{
		Type:    "uv",
		Path:    filepath.Dir(file.Path),
		Name:    "uv.toml",
		Version: doc.RequiredVersion,
	}}, nil
}
