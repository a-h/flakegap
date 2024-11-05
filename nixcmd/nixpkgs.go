package nixcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func GetNixpkgsReferences(r io.Reader) (references []string, err error) {
	var nfl map[string]any
	if err = json.NewDecoder(r).Decode(&nfl); err != nil {
		return references, err
	}
	return getNixpkgsRefs(nfl)
}

func getNixpkgsRefs(m map[string]any) (refs []string, err error) {
	for k, v := range m {
		v, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if k == "locked" {
			lref, err := getStrings(v, "type", "owner", "repo", "rev")
			if err != nil {
				return refs, err
			}
			if !(strings.EqualFold(lref[0], "github") && strings.EqualFold(lref[1], "NixOS") && strings.EqualFold(lref[2], "nixpkgs")) {
				continue
			}
			refs = append(refs, fmt.Sprintf("%s:%s/%s/%s", lref[0], lref[1], lref[2], lref[3]))
			continue
		}
		ll, err := getNixpkgsRefs(v)
		if err != nil {
			return refs, err
		}
		refs = append(refs, ll...)
	}
	return refs, nil
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
