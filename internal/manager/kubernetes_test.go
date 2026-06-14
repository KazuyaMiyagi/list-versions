package manager

import (
	"path/filepath"
	"sort"
	"testing"
)

func TestKubernetes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "manifests/app.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      initContainers:
        - name: migrate
          image: myorg/migrate:1.2.0
      containers:
        - name: api
          image: myorg/api:3.4.0
        - name: sidecar
          image: datadog/agent:7.50.0
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: cleanup
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: cleanup
              image: alpine:3.19
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  key: value
`)
	writeFile(t, filepath.Join(dir, "manifests/pod.yaml"), `
apiVersion: v1
kind: Pod
metadata:
  name: dev
spec:
  containers:
    - image: nginx:1.25.3
`)
	// Should not be picked up by Kubernetes manager.
	writeFile(t, filepath.Join(dir, ".github/workflows/ci.yml"), `
jobs:
  x:
    steps:
      - uses: actions/checkout@v4
`)

	files := matched(t, Kubernetes{}, dir)
	if len(files) != 2 {
		t.Fatalf("want 2 manifest files (excluding .github/workflows), got %d (%v)", len(files), files)
	}

	var allImages []string
	for _, e := range collect(t, Kubernetes{}, dir) {
		if e.Type != "Kubernetes" {
			t.Errorf("unexpected type %q", e.Type)
		}
		allImages = append(allImages, e.Version)
	}
	sort.Strings(allImages)
	want := []string{
		"alpine:3.19",
		"datadog/agent:7.50.0",
		"myorg/api:3.4.0",
		"myorg/migrate:1.2.0",
		"nginx:1.25.3",
	}
	if len(allImages) != len(want) {
		t.Fatalf("got %d images (%v), want %d (%v)", len(allImages), allImages, len(want), want)
	}
	for i, w := range want {
		if allImages[i] != w {
			t.Errorf("[%d] got %q, want %q", i, allImages[i], w)
		}
	}
}

func TestKubernetes_IgnoresInvalidYaml(t *testing.T) {
	dir := t.TempDir()
	// Helm template - has Go template directives -> invalid YAML.
	writeFile(t, filepath.Join(dir, "templates/deploy.yaml"), `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  template:
    spec:
      containers:
        - image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
`)
	files := matched(t, Kubernetes{}, dir)
	if len(files) != 1 {
		t.Fatalf("want 1 file detected, got %d", len(files))
	}
	es := collect(t, Kubernetes{}, dir)
	if len(es) != 0 {
		t.Errorf("Helm template should be skipped silently, got %+v", es)
	}
}
