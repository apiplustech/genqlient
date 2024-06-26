package graphql

import (
	"encoding/json"
	"io"
)

type Upload struct {
	Filename string
	MIMEType string
	Body     io.Reader
}

// MarshalJSON implements json.Marshaler.
func (Upload) MarshalJSON() ([]byte, error) {
	return json.Marshal(nil)
}
