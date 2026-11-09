package domain

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("not found")

type Repository interface {
	UpsertTelegramUser(ctx context.Context, telegramID int64, username string) (User, error)
	ListSpecializations(ctx context.Context) ([]Specialization, error)
	GetSpecializationBySlug(ctx context.Context, slug string) (Specialization, error)
	CreateSession(ctx context.Context, session InterviewSession) (InterviewSession, error)
	GetSession(ctx context.Context, sessionID int64) (InterviewSession, error)
	GetActiveSessionByUser(ctx context.Context, userID int64) (InterviewSession, error)
	UpdateSession(ctx context.Context, session InterviewSession) error
	SaveAnswer(ctx context.Context, answer InterviewAnswer) (InterviewAnswer, error)
	ListAnswers(ctx context.Context, sessionID int64) ([]InterviewAnswer, error)
	ListSessionsByUser(ctx context.Context, userID int64, limit int) ([]InterviewSession, error)
}
