package resources

import "time"

// ChatMessage uses the same JSON shape as a live world-chat frame so clients
// can render persisted and real-time messages through one code path.
type ChatMessage struct {
	ID        int64     `json:"id,string"`
	UserID    int64     `json:"userId,string"`
	Username  string    `json:"username"`
	FullName  string    `json:"fullName"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}
