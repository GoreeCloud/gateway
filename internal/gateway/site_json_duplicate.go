package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
)

// rejectDuplicateSiteFields refuses ambiguous top-level route properties.
// The caller still runs strict Site decoding with unknown-field rejection.
func rejectDuplicateSiteFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return errors.New("Invalid site JSON")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return errors.New("Invalid site JSON")
		}
		key, ok := token.(string)
		if !ok {
			return errors.New("Invalid site JSON")
		}
		if seen[key] {
			return errors.New("Duplicate site JSON field")
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return errors.New("Invalid site JSON")
		}
	}
	if _, err := decoder.Token(); err != nil {
		return errors.New("Invalid site JSON")
	}
	return nil
}
