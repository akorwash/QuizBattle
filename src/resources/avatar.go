package resources

import "time"

// UserAvatar describes the canonical image produced after an upload. The URL
// is stable; clients use ETag revalidation to observe replacements.
type UserAvatar struct {
	UserID      int64     `json:"userId,string"`
	URL         string    `json:"url"`
	ETag        string    `json:"etag"`
	ContentType string    `json:"contentType"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	ByteSize    int64     `json:"byteSize"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// UserAvatarContent is returned by the application layer to the HTTP
// transport. Data is deliberately excluded from JSON responses.
type UserAvatarContent struct {
	UserID      int64
	Data        []byte
	ETag        string
	ContentType string
	UpdatedAt   time.Time
}
