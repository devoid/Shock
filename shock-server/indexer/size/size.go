package size

import (
	"github.com/MG-RAST/Shock/shock-server/node/file/index"
	"io"
)

type indexer struct{}

func NewIndexer(f io.ReadCloser) index.Indexer {
	return &indexer{}
}

func (i *indexer) Create() (count int64, err error) {
	return
}

func (i *indexer) Dump(f string) error {
	return nil
}

func (i *indexer) Close() (err error) {
	return
}
