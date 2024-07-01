package graphql

import (
	"io"
)

type Upload struct {
	FileName string
	Body     io.Reader
}
