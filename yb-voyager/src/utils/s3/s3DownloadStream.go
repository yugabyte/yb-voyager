package s3

import "io"

type S3DownloadStream struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	offset int64
}

func NewS3DownloadStream() *S3DownloadStream {
	reader, writer := io.Pipe()

	return &S3DownloadStream{
		reader: reader,
		writer: writer,
		offset: 0,
	}
}

func (s *S3DownloadStream) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *S3DownloadStream) WriteAt(p []byte, off int64) (int, error) {
	if s.offset != off {
		return 0, io.EOF
	}

	n, err := s.writer.Write(p)
	if err != nil {
		return n, err
	}
	s.offset += int64(n)

	return n, nil
}

func (s *S3DownloadStream) Close() error {
	return s.writer.Close()
}

func (s *S3DownloadStream) CloseWithError(err error) error {
	return s.writer.CloseWithError(err)
}
