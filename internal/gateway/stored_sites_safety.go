package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// rejectAmbiguousStoredSites applies the same unambiguous property rules to
// private route state on restart as to HTTP site mutations. A failing check
// never feeds a partially interpreted configuration to the Caddy loader.
// Callers already bound private state files to 256 KiB before this check.
func rejectAmbiguousStoredSites(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var items []json.RawMessage
	if err := decoder.Decode(&items); err != nil || items == nil {
		return errors.New("managed sites state must be a JSON array")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("managed sites state has trailing content")
	}
	for _, item := range items {
		if err := rejectDuplicateSiteFields(item); err != nil {
			return errors.New("managed sites state contains ambiguous site fields")
		}
	}
	return nil
}
