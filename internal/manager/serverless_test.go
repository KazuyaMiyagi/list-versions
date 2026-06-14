package manager

import (
	"path/filepath"
	"testing"
)

func TestServerless(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "serverless.yml"), `
service: my-service
provider:
  name: aws
  runtime: nodejs22.x
functions:
  api:
    handler: handler.api
  worker:
    handler: handler.worker
    runtime: python3.13
`)
	files := matched(t, Serverless{}, dir)
	if len(files) != 1 {
		t.Fatalf("detect: %v", files)
	}
	es := collect(t, Serverless{}, dir)
	got := map[string]string{}
	for _, e := range es {
		got[e.Name] = e.Version
	}
	want := map[string]string{
		"serverless.yml:provider":         "nodejs22.x",
		"serverless.yml:functions.worker": "python3.13",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["serverless.yml:functions.api"]; ok {
		t.Errorf("api should not emit (no runtime override)")
	}
}
