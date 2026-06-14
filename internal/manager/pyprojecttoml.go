package manager

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// PyprojectToml reads the Python version constraint from pyproject.toml,
// preferring PEP 621 `requires-python` and falling back to Poetry's `python`
// dependency.
type PyprojectToml struct{}

func (PyprojectToml) Name() string { return "pyproject-toml" }

func (PyprojectToml) Match(path string) bool { return filepath.Base(path) == "pyproject.toml" }

type pyprojectDoc struct {
	Project struct {
		RequiresPython string `toml:"requires-python"`
	} `toml:"project"`
	Tool struct {
		Poetry struct {
			Dependencies map[string]toml.Primitive `toml:"dependencies"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

func (PyprojectToml) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	var doc pyprojectDoc
	md, err := toml.Decode(string(data), &doc)
	if err != nil {
		return nil, err
	}
	ver := doc.Project.RequiresPython
	if ver == "" {
		if p, ok := doc.Tool.Poetry.Dependencies["python"]; ok {
			var s string
			if err := md.PrimitiveDecode(p, &s); err == nil {
				ver = s
			} else {
				var t struct {
					Version string `toml:"version"`
				}
				_ = md.PrimitiveDecode(p, &t)
				ver = t.Version
			}
		}
	}
	if ver == "" {
		return nil, nil
	}
	return []entry.Entry{{
		Type:    "Python",
		Path:    filepath.Dir(file.Path),
		Name:    "pyproject.toml",
		Version: ver,
	}}, nil
}
