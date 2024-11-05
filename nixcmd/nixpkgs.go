package nixcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func GetNixpkgsReference(r io.Reader) (ref string, err error) {
	var nfl map[string]any
	if err = json.NewDecoder(r).Decode(&nfl); err != nil {
		return ref, err
	}
	return getNixpkgsRef(nfl)
}

func JSONMapValue[T any](m map[string]any, keys ...string) (v T, ok bool) {
	if len(keys) == 0 {
		return v, false
	}
	for _, k := range keys[:len(keys)-1] {
		m, ok = m[k].(map[string]any)
		if !ok {
			return v, false
		}
	}
	v, ok = m[keys[len(keys)-1]].(T)
	return v, ok
}

func getNixpkgsRef(m map[string]any) (ref string, err error) {
	rootName, ok := JSONMapValue[string](m, "root")
	if !ok {
		return ref, errors.New("root key not present")
	}
	nixpkgsNodeName, ok := JSONMapValue[string](m, "nodes", rootName, "inputs", "nixpkgs")
	if !ok {
		return ref, fmt.Errorf("nixpkgs name not found in root inputs %s", rootName)
	}
	jsonPath := []string{"nodes", nixpkgsNodeName, "locked"}
	nixpkgsNode, ok := JSONMapValue[map[string]any](m, jsonPath...)
	if !ok {
		return ref, fmt.Errorf("nixpkgs node not found in %s", nixpkgsNodeName)
	}
	lref, err := getStrings(nixpkgsNode, "type", "owner", "repo", "rev")
	if err != nil {
		return ref, err
	}
	return fmt.Sprintf("%s:%s/%s/%s", lref[0], lref[1], lref[2], lref[3]), nil
}

func getStrings(m map[string]any, keys ...string) (values []string, err error) {
	for _, k := range keys {
		v, ok := m[k].(string)
		if !ok {
			return nil, fmt.Errorf("%s not found in map", k)
		}
		values = append(values, v)
	}
	return values, nil
}
