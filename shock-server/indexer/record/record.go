package record

import (
	"github.com/MG-RAST/Shock/shock-server/node/file/format/multi"
	"github.com/MG-RAST/Shock/shock-server/node/file/format/seq"
	"github.com/MG-RAST/Shock/shock-server/node/file/index"
	"io"
	"os"
)

type indexer struct {
	f     *os.File
	r     seq.Reader
	Index *index.Idx
}

func NewIndexer(f *os.File) index.Indexer {
	return &indexer{
		f:     f,
		r:     multi.NewReader(f),
		Index: index.New(),
	}
}

func (i *indexer) Create() (count int64, err error) {
	curr := int64(0)
	count = 0
	for {
		buf := make([]byte, 32*1024)
		n, er := i.r.ReadRaw(buf)
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		i.Index.Append([]int64{curr, int64(n)})
		curr += int64(n)
		count += 1
	}
	return
}

func (i *indexer) Dump(f string) error {
	return i.Index.Dump(f)
}

func (i *indexer) Close() (err error) {
	i.f.Close()
	return
}
