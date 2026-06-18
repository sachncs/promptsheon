package api

import (
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/webhook"
)

func (s *Server) handleListPendingReviews(w http.ResponseWriter, r *http.Request) error {
	reviews, err := s.db.ListPendingReviews(r.Context())
	if err != nil {
		return err
	}
	if reviews == nil {
		reviews = []*models.Review{}
	}
	writeJSON(w, http.StatusOK, reviews)
	return nil
}

func (s *Server) handleCreateReview(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		ResourceID     string `json:"resource_id"`
		ResourceType   string `json:"resource_type"`
		Author         string `json:"author"`
		QuorumRequired int    `json:"quorum_required,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.ResourceID == "" || req.ResourceType == "" || req.Author == "" {
		return ErrBadRequest
	}
	if req.QuorumRequired <= 0 {
		req.QuorumRequired = 1
	}

	review := &models.Review{
		ID:             generateID(),
		ResourceID:     req.ResourceID,
		ResourceType:   req.ResourceType,
		Author:         req.Author,
		Status:         models.ReviewPending,
		QuorumRequired: req.QuorumRequired,
		ApprovalsCount: 0,
		Comments:       []models.Comment{},
		CreatedAt:      time.Now(),
	}

	if err := s.db.CreateReview(r.Context(), review); err != nil {
		return err
	}
	s.audit(r.Context(), "create", "review:"+review.ID, map[string]any{"resource": review.ResourceType + ":" + review.ResourceID, "author": review.Author})
	writeJSON(w, http.StatusCreated, review)
	return nil
}

func (s *Server) handleApproveReview(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	review, err := s.db.GetReview(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if review.Status != models.ReviewPending {
		return ErrConflict
	}

	now := time.Now()
	review.Status = models.ReviewApproved
	review.ResolvedAt = &now
	review.ApprovalsCount++

	if err := s.db.UpdateReview(r.Context(), review); err != nil {
		return err
	}

	// Check quorum: count all approved reviews for this resource
	reviews, _ := s.db.ListReviewsByResource(r.Context(), review.ResourceID, review.ResourceType)
	approvedCount := 0
	for _, rv := range reviews {
		if rv.Status == models.ReviewApproved {
			approvedCount++
		}
	}
	quorumMet := approvedCount >= review.QuorumRequired

	// Auto-transition resource status if quorum is met
	if quorumMet {
		if review.ResourceType == "prompt" {
			if prompt, err := s.db.GetPrompt(r.Context(), review.ResourceID); err == nil {
				prompt.Status = models.StatusApproved
				prompt.UpdatedAt = now
				if err := s.db.UpdatePrompt(r.Context(), prompt); err != nil {
					s.logger.Error("failed to auto-approve prompt", "err", err, "prompt_id", prompt.ID)
				}
				s.audit(r.Context(), "auto_approve", "prompt:"+prompt.ID, map[string]any{
					"review_id": review.ID,
					"approvals": approvedCount,
					"quorum":    review.QuorumRequired,
				})
			}
		}

		if review.ResourceType == "agent" {
			if agent, err := s.db.GetAgent(r.Context(), review.ResourceID); err == nil {
				agent.Status = models.StatusApproved
				agent.UpdatedAt = now
				if err := s.db.UpdateAgent(r.Context(), agent); err != nil {
					s.logger.Error("failed to auto-approve agent", "err", err, "agent_id", agent.ID)
				}
				s.audit(r.Context(), "auto_approve", "agent:"+agent.ID, map[string]any{
					"review_id": review.ID,
					"approvals": approvedCount,
					"quorum":    review.QuorumRequired,
				})
			}
		}
	}

	s.audit(r.Context(), "approve", "review:"+review.ID, map[string]any{"resource": review.ResourceType + ":" + review.ResourceID})

	// Emit webhook event
	if s.webhooks != nil {
		s.webhooks.Emit(webhook.Event{
			ID:        generateID(),
			Type:      webhook.EventReviewApproved,
			Resource:  review.ResourceType + ":" + review.ResourceID,
			Data:      map[string]any{"review_id": review.ID, "author": review.Author},
			Timestamp: now,
		})
	}

	writeJSON(w, http.StatusOK, review)
	return nil
}

func (s *Server) handleRejectReview(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	review, err := s.db.GetReview(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if review.Status != models.ReviewPending {
		return ErrConflict
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}

	now := time.Now()
	review.Status = models.ReviewRejected
	review.ResolvedAt = &now
	review.Comments = append(review.Comments, models.Comment{
		ID:        generateID(),
		Content:   "Rejection reason: " + req.Reason,
		CreatedAt: now,
	})

	if err := s.db.UpdateReview(r.Context(), review); err != nil {
		return err
	}
	s.audit(r.Context(), "reject", "review:"+review.ID, map[string]any{"resource": review.ResourceType + ":" + review.ResourceID, "reason": req.Reason})

	// Emit webhook event
	if s.webhooks != nil {
		s.webhooks.Emit(webhook.Event{
			ID:        generateID(),
			Type:      webhook.EventReviewRejected,
			Resource:  review.ResourceType + ":" + review.ResourceID,
			Data:      map[string]any{"review_id": review.ID, "reason": req.Reason},
			Timestamp: now,
		})
	}

	writeJSON(w, http.StatusOK, review)
	return nil
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	review, err := s.db.GetReview(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	var req struct {
		UserID  string `json:"user_id"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Content == "" {
		return ErrBadRequest
	}

	review.Comments = append(review.Comments, models.Comment{
		ID:        generateID(),
		UserID:    req.UserID,
		Content:   req.Content,
		CreatedAt: time.Now(),
	})

	if err := s.db.UpdateReview(r.Context(), review); err != nil {
		return err
	}
	s.audit(r.Context(), "comment", "review:"+review.ID, map[string]any{"user_id": req.UserID})
	writeJSON(w, http.StatusOK, review)
	return nil
}
