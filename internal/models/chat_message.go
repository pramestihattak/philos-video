package models

import "time"

type ChatMessage struct {
	ID        string    `json:"id"`
	StreamID  string    `json:"stream_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	UserPic   string    `json:"user_picture,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
