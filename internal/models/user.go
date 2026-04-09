package models

import "time"

type User struct {
	ID                string    `json:"id"`
	GoogleSub         string    `json:"google_sub,omitempty"`
	Email             string    `json:"email"`
	Name              string    `json:"name,omitempty"`
	Picture           string    `json:"picture,omitempty"`
	UploadQuotaBytes  int64     `json:"upload_quota_bytes"`
	UsedBytes         int64     `json:"used_bytes"`
	CreatedAt         time.Time `json:"created_at"`
}
