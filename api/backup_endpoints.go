package api

import (
	"fmt"
	"github.com/pkg/errors"
)

type BackupRemoteUploadResponse struct {
	CompleteMultipartUpload string
	AbortMultipartUpload    string
	Parts                   []string
	PartSize                int64
}

func (r *Request) GetBackupRemoteUploadURLs(backup string, size int64) (*BackupRemoteUploadResponse, error) {
	resp, err := r.Get(fmt.Sprintf("/backups/%s?size=%d", backup, size), nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resp.Body.Close()

	if resp.HasError() {
		return nil, resp.Error()
	}

	var res BackupRemoteUploadResponse
	if err := resp.Bind(&res); err != nil {
		return nil, errors.WithStack(err)
	}

	return &res, nil
}

type BackupRequest struct {
	Checksum     string `json:"checksum"`
	ChecksumType string `json:"checksum_type"`
	Size         int64  `json:"size"`
	Successful   bool   `json:"successful"`
}

// Notifies the panel that a specific backup has been completed and is now
// available for a user to view and download.
func (r *Request) SendBackupStatus(backup string, data BackupRequest) error {
	resp, err := r.Post(fmt.Sprintf("/backups/%s", backup), data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.HasError() {
		return resp.Error()
	}

	return nil
}
