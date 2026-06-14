package manager

import (
	"path/filepath"
	"sort"
	"testing"
)

func TestDockerfile_Detect(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Dockerfile"), "FROM alpine:3\n")
	writeFile(t, filepath.Join(dir, "Dockerfile.prod"), "FROM ruby:3.3\n")
	writeFile(t, filepath.Join(dir, "MyDockerfile"), "FROM nope:1\n")
	files := matched(t, Dockerfile{}, dir)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d (%v)", len(files), files)
	}
}

func TestDockerfile_Extract(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Dockerfile"), `
ARG RUBY_VERSION=3.3.0
ARG NODE_TAG="20.10.0"
ARG UNRESOLVED
FROM --platform=$BUILDPLATFORM ruby:${RUBY_VERSION} AS build
FROM ruby:3.3.0 AS prod
FROM node:${NODE_TAG}@sha256:abcdef
FROM scratch
FROM $UNRESOLVED
FROM ${BASE}:1.0
FROM public.ecr.aws/lambda/ruby:4.0.5
FROM registry.example.com:5000/myorg/app:v1.2.3
`)
	es := collect(t, Dockerfile{}, dir)

	gotVersions := map[string]bool{}
	for _, e := range es {
		if e.Name != "Dockerfile" {
			t.Errorf("expected Name=Dockerfile, got %q", e.Name)
		}
		gotVersions[e.Version] = true
	}
	want := []string{
		"ruby:3.3.0",
		"node:20.10.0@sha256:abcdef",
		"public.ecr.aws/lambda/ruby:4.0.5",
		"registry.example.com:5000/myorg/app:v1.2.3",
	}
	if len(gotVersions) != len(want) {
		got := make([]string, 0, len(gotVersions))
		for v := range gotVersions {
			got = append(got, v)
		}
		sort.Strings(got)
		t.Errorf("got %d entries (%v), want %d (%v)", len(gotVersions), got, len(want), want)
	}
	for _, v := range want {
		if !gotVersions[v] {
			t.Errorf("missing version %q", v)
		}
	}
}
