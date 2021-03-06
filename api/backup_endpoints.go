package api

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
)

type BackupRequest struct {
	Checksum     string `json:"checksum"`
	ChecksumType string `json:"checksum_type"`
	Size         int64  `json:"size"`
	Successful   bool   `json:"successful"`
}

// Notifies the panel that a specific backup has been completed and is now
// available for a user to view and download.
func (r *Request) SendBackupStatus(backup string, data BackupRequest) error {
	b, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	resp, err := r.Post(fmt.Sprintf("/backups/%s", backup), b)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	return resp.Error()
}
