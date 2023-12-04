/*
Command release publishes a new GitHub release with a given set of
files.

It expects an environment variables with the name GITHUB_REF_NAME. The
value of GITHUB_REF_NAME must be a git tag with the format
"dir/semver" (e.g. "checktypes/v1.2.3").

For a given tag, it creates three releases:

  - dir/vMAJOR.MINOR.PATCH
  - dir/vMAJOR.MINOR
  - dir/vMAJOR

The regular files in the directory specified in the tag are attached
to all the releases.

For instance, if the tag is "checktypes/v1.2.3", the following
releases would be created:

  - checktypes/v1.2.3 (unique)
  - checktypes/v1.2 (updated if it already exists)
  - checktypes/v1 (updated if it already exists)

And the files "checktypes/*" would be attached to them.

This release schema allows users to specify versions depending on
their needs. In other words,

  - v1.2.3  :=  ==v1.2.3
  - v1.2    :=  >=v1.2.0, <v1.3.0
  - v1      :=  >=v1.0.0, <v2.0.0
  - v0.2.3  :=  ==v0.2.3
  - v0.2    :=  >=v0.2.0, <v0.3.0
  - v0      :=  >=v0.0.0, <v1.0.0
*/
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

func main() {
	log.SetFlags(0)

	refName := os.Getenv("GITHUB_REF_NAME")
	if refName == "" {
		log.Fatalf("error: missing env var GITHUB_REF_NAME")
	}

	parts := strings.Split(refName, "/")
	if len(parts) != 2 {
		log.Fatalf("error: invalid tag name: %v", refName)
	}
	dir := parts[0]
	version := parts[1]

	if !semver.IsValid(version) {
		log.Fatalf("error: invalid version: %v", version)
	}

	files, err := readDir(dir)
	if err != nil {
		log.Fatalf("error: list files: %v", err)
	}

	hash, err := gitHash(refName)
	if err != nil {
		log.Fatalf("error: get hash: %v", err)
	}

	for _, arg := range []struct {
		version string
		delete  bool
	}{
		{semver.Major(version), true},
		{semver.MajorMinor(version), true},
		{version, false},
	} {
		tag := dir + "/" + arg.version
		if err := ghRelease(tag, hash, arg.delete, files); err != nil {
			log.Fatalf("error: create GitHub release %q: %v", tag, err)
		}
	}
}

func gitHash(ref string) (string, error) {
	hash, err := execCmd("git", "show-ref", "--hash", ref)
	if err != nil {
		return "", fmt.Errorf("git show-ref: %w", err)
	}
	return hash, nil
}

func ghRelease(tag, target string, delete bool, files []string) error {
	if delete {
		if _, err := execCmd("gh", "release", "delete", "--cleanup-tag", "--yes", tag); err != nil {
			log.Printf("warn: could not delete release %q", tag)
		}
	}

	args := []string{"release", "create", "--target", target, tag}
	args = append(args, files...)
	if _, err := execCmd("gh", args...); err != nil {
		return fmt.Errorf("gh release create (%#v): %w", args, err)
	}

	return nil
}

func readDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			log.Printf("warn: skipping dir %q", entry.Name())
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	return files, nil
}

func execCmd(name string, arg ...string) (string, error) {
	stderr := &bytes.Buffer{}
	cmd := exec.Command(name, arg...)
	cmd.Stderr = stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cmd output: %w: %#q", err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}
