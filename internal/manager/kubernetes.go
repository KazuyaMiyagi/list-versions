package manager

import (
	"bytes"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KazuyaMiyagi/list-versions/internal/entry"
)

// underGithubWorkflows reports whether path lives under a `.github/workflows/`
// directory, which the GitHub Actions manager owns. The Kubernetes and
// CloudFormation managers match YAML broadly and use this to step aside.
func underGithubWorkflows(path string) bool {
	dir := filepath.Dir(path)
	return filepath.Base(dir) == "workflows" && filepath.Base(filepath.Dir(dir)) == ".github"
}

// Kubernetes pulls container images out of K8s manifests (Pod / Deployment /
// StatefulSet / DaemonSet / ReplicaSet / Job / CronJob). It tolerates
// multi-document YAML and silently skips files it can't parse (Helm templates
// with `{{ }}` blocks are intentionally ignored because they aren't valid
// YAML before rendering).
type Kubernetes struct{}

func (Kubernetes) Name() string { return "kubernetes" }

func (Kubernetes) Match(path string) bool {
	name := filepath.Base(path)
	if !(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
		return false
	}
	// `.github/workflows/*.yml` belongs to the GitHub Actions manager.
	return !underGithubWorkflows(path)
}

// k8sKinds is the set of `kind:` values whose pod-spec we know how to read.
// CronJob has an extra `jobTemplate` indirection; everything else is either a
// Pod or a controller whose `template.spec` is a PodSpec.
var k8sKinds = map[string]bool{
	"Pod":         true,
	"Deployment":  true,
	"StatefulSet": true,
	"DaemonSet":   true,
	"ReplicaSet":  true,
	"Job":         true,
	"CronJob":     true,
}

func (Kubernetes) Extract(file File) ([]entry.Entry, error) {
	data, err := readBytes(file)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	dir := filepath.Dir(file.Path)
	base := filepath.Base(file.Path)
	var entries []entry.Entry
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			break
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
			continue
		}
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			continue
		}
		apiVersion := scalarChild(root, "apiVersion")
		kind := scalarChild(root, "kind")
		if apiVersion == "" || !k8sKinds[kind] {
			continue
		}
		name := scalarChild(findMapChild(root, "metadata"), "name")
		label := kind
		if name != "" {
			label = kind + "/" + name
		}
		for _, img := range extractK8sImages(kind, root) {
			entries = append(entries, entry.Entry{
				Type:    "Kubernetes",
				Path:    dir,
				Name:    base + ":" + label,
				Version: img,
			})
		}
	}
	return entries, nil
}

// extractK8sImages returns container + initContainer images from a single
// manifest root node, normalising over the kind-specific path to a PodSpec.
func extractK8sImages(kind string, root *yaml.Node) []string {
	podSpec := podSpecFor(kind, root)
	if podSpec == nil {
		return nil
	}
	var images []string
	for _, blockKey := range []string{"containers", "initContainers"} {
		seq := findMapChild(podSpec, blockKey)
		if seq == nil || seq.Kind != yaml.SequenceNode {
			continue
		}
		for _, c := range seq.Content {
			if img := scalarChild(c, "image"); img != "" {
				images = append(images, img)
			}
		}
	}
	return images
}

func podSpecFor(kind string, root *yaml.Node) *yaml.Node {
	switch kind {
	case "Pod":
		return findMapChild(root, "spec")
	case "CronJob":
		spec := findMapChild(root, "spec")
		jt := findMapChild(spec, "jobTemplate")
		tpl := findMapChild(jt, "spec")
		tpl = findMapChild(tpl, "template")
		return findMapChild(tpl, "spec")
	default:
		spec := findMapChild(root, "spec")
		tpl := findMapChild(spec, "template")
		return findMapChild(tpl, "spec")
	}
}

func scalarChild(n *yaml.Node, key string) string {
	c := findMapChild(n, key)
	if c == nil || c.Kind != yaml.ScalarNode {
		return ""
	}
	return c.Value
}
