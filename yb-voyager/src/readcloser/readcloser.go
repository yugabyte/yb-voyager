package readcloser

type ReadCloser interface {
	Close() error
	ReadString(byte) (string, error)
	Open(string) error
}

func NewReadCloser(datafile string) ReadCloser {
	return NewTextReadCloser()
}
