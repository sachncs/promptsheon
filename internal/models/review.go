package models

import "time"

// ReviewStatus represents the state of a review.
type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

// Comment is a single comment on a review.
type Comment struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Review represents an approval workflow entry for a prompt or agent.
type Review struct {
	ID             string       `json:"id"`
	ResourceID     string       `json:"resource_id"`
	ResourceType   string       `json:"resource_type"` // "prompt", "agent"
	Author         string       `json:"author"`
	Status         ReviewStatus `json:"status"`
	Comments       []Comment    `json:"comments"`
	QuorumRequired int          `json:"quorum_required"` // number of approvals needed
	ApprovalsCount int          `json:"approvals_count"` // current approval count
	CreatedAt      time.Time    `json:"created_at"`
	ResolvedAt     *time.Time   `json:"resolved_at,omitempty"`
}
