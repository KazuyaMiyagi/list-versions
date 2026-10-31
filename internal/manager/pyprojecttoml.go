package manager

import (
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// PyprojectToml reads the Python version constraint from pyproject.toml,
// preferring PEP 621 `requires-python` and falling back to Poetry's `python`
// dependency. It also reads the package manager requirements
// `[tool.poetry] requires-poetry` and `[tool.uv] required-version`.
type PyprojectToml struct{}

func (PyprojectToml) Name() string { return "pyproject-toml" }

func (PyprojectToml) Match(path string) bool { return filepath.Base(path) == "pyproject.toml" }

type pyprojectDoc struct {
	Project struct {
		RequiresPython string `toml:"requires-python"`
	} `toml:"project"`
	Tool struct {
		Poetry struct {
			RequiresPoetry string                    `toml:"requires-poetry"`
			Dependencies   map[string]toml.Primitive `toml:"dependencies"`
		} `toml:"poetry"`
		Uv struct {
			RequiredVersion string `toml:"required-version"`
		} `toml:"uv"`
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
	dir := filepath.Dir(file.Path)
	var entries []entry.Entry
	if ver != "" {
		entries = append(entries, entry.Entry{Type: "Python", Path: dir, Name: "pyproject.toml", Version: ver})
	}
	if v := doc.Tool.Poetry.RequiresPoetry; v != "" {
		entries = append(entries, entry.Entry{Type: "Poetry", Path: dir, Name: "pyproject.toml", Version: v})
	}
	if v := doc.Tool.Uv.RequiredVersion; v != "" {
		entries = append(entries, entry.Entry{Type: "uv", Path: dir, Name: "pyproject.toml", Version: v})
	}
	return entries, nil
}
