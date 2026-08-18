package nixcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nix-community/go-nix/pkg/narinfo"
)

// sortStorePathsByDependency reads all narinfo files from a Nix binary cache
// directory and returns the store paths in topological order, prerequisites first.
func sortStorePathsByDependency(cacheDir string) (paths []string, err error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	storePathToRefs := make(map[string][]string)
	baseNameToStorePath := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".narinfo") {
			continue
		}
		ni, err := parseNarinfo(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", entry.Name(), err)
		}
		if ni.StorePath == "" {
			continue
		}
		baseNameToStorePath[filepath.Base(ni.StorePath)] = ni.StorePath
		storePathToRefs[ni.StorePath] = ni.References
	}

	// Resolve base names to full store paths, keeping only refs present in this
	// cache and skipping self-references.
	storePathToPrerequisites := make(map[string][]string, len(storePathToRefs))
	for storePath, refs := range storePathToRefs {
		var resolved []string
		for _, ref := range refs {
			if full, ok := baseNameToStorePath[ref]; ok && full != storePath {
				resolved = append(resolved, full)
			}
		}
		storePathToPrerequisites[storePath] = resolved
	}

	return topoSortPaths(storePathToPrerequisites)
}

func parseNarinfo(path string) (ni *narinfo.NarInfo, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return narinfo.Parse(f)
}

// topoSortPaths performs a topological sort using Kahn's algorithm.
// storePathToPrerequisites maps each node to the nodes it depends on (which must come first).
func topoSortPaths(storePathToPrerequisites map[string][]string) ([]string, error) {
	storePathToInDegree := make(map[string]int, len(storePathToPrerequisites))
	storePathToDependents := make(map[string][]string)

	for node := range storePathToPrerequisites {
		if _, ok := storePathToInDegree[node]; !ok {
			storePathToInDegree[node] = 0
		}
	}
	for node, prerequisites := range storePathToPrerequisites {
		for _, prereq := range prerequisites {
			if _, ok := storePathToInDegree[prereq]; !ok {
				continue // prereq not in our set; treat as already satisfied
			}
			storePathToDependents[prereq] = append(storePathToDependents[prereq], node)
			storePathToInDegree[node]++
		}
	}

	var queue []string
	for node, degree := range storePathToInDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}
	slices.Sort(queue)

	result := make([]string, 0, len(storePathToPrerequisites))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)
		var ready []string
		for _, dependent := range storePathToDependents[node] {
			storePathToInDegree[dependent]--
			if storePathToInDegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		slices.Sort(ready)
		queue = append(queue, ready...)
	}

	if len(result) != len(storePathToPrerequisites) {
		return nil, fmt.Errorf("dependency cycle detected in nix store paths")
	}

	return result, nil
}
