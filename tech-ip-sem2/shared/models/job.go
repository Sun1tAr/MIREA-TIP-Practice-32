package models

// JobMessage - структура задачи для очереди
type JobMessage struct {
	Job       string `json:"job"`
	TaskID    string `json:"task_id"`
	Attempt   int    `json:"attempt"`
	MessageID string `json:"message_id"`
	CreatedAt string `json:"created_at,omitempty"`
}
