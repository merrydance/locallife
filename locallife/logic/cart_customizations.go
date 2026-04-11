package logic

import (
	"encoding/json"
	"io"
	"sort"
	"strings"
)

// MarshalCustomizationsCanonical produces deterministic JSON for cart customizations.
func MarshalCustomizationsCanonical(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	var b strings.Builder
	if err := writeCanonicalJSON(&b, v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func writeCanonicalJSON(w io.StringWriter, v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if _, err := w.WriteString("{"); err != nil {
			return err
		}
		for i, k := range keys {
			if i > 0 {
				if _, err := w.WriteString(","); err != nil {
					return err
				}
			}
			keyBytes, _ := json.Marshal(k)
			if _, err := w.WriteString(string(keyBytes)); err != nil {
				return err
			}
			if _, err := w.WriteString(":"); err != nil {
				return err
			}
			if err := writeCanonicalJSON(w, val[k]); err != nil {
				return err
			}
		}
		_, err := w.WriteString("}")
		return err
	case []interface{}:
		if _, err := w.WriteString("["); err != nil {
			return err
		}
		for i, elem := range val {
			if i > 0 {
				if _, err := w.WriteString(","); err != nil {
					return err
				}
			}
			if err := writeCanonicalJSON(w, elem); err != nil {
				return err
			}
		}
		_, err := w.WriteString("]")
		return err
	default:
		encoded, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = w.WriteString(string(encoded))
		return err
	}
}
