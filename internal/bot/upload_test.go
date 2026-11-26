package bot

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeProcess struct{ waitErr error }

func (fakeProcess) Close() error  { return nil }
func (p fakeProcess) Wait() error { return p.waitErr }

func TestUploadStreamErrors(t *testing.T) {
	uploadFailed := errors.New("upload failed")
	downloadFailed := errors.New("download failed")

	tests := []struct {
		name         string
		readAll      bool
		uploadErr    error
		waitErr      error
		wantUpload   error
		wantDownload error
	}{
		{name: "success", readAll: true},
		{name: "empty download rejected by telegram", readAll: true, uploadErr: uploadFailed, waitErr: downloadFailed, wantDownload: downloadFailed},
		{name: "download failed, upload accepted", readAll: true, waitErr: downloadFailed, wantDownload: downloadFailed},
		{name: "upload failed after full download", readAll: true, uploadErr: uploadFailed, wantUpload: uploadFailed},
		{name: "upload aborted early", uploadErr: uploadFailed, waitErr: downloadFailed, wantUpload: uploadFailed, wantDownload: downloadFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uploadErr, downloadErr := uploadStream(strings.NewReader(""), fakeProcess{test.waitErr}, func(r io.Reader) error {
				if test.readAll {
					_, _ = io.Copy(io.Discard, r)
				}
				return test.uploadErr
			})
			if uploadErr != test.wantUpload || downloadErr != test.wantDownload {
				t.Fatalf("got (%v, %v), want (%v, %v)", uploadErr, downloadErr, test.wantUpload, test.wantDownload)
			}
		})
	}
}
