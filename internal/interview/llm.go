package interview

import (
	"context"

	"ai-interview-coach/internal/domain"
)

type LLMClient interface {
	GenerateQuestion(ctx context.Context, systemPrompt string, req domain.QuestionRequest) (domain.GeneratedQuestion, error)
	EvaluateAnswer(ctx context.Context, systemPrompt string, session domain.InterviewSession, history []domain.InterviewAnswer, answer string) (domain.Evaluation, error)
	GenerateFinalFeedback(ctx context.Context, systemPrompt string, session domain.InterviewSession, answers []domain.InterviewAnswer, aggregate domain.Feedback) (domain.Feedback, error)
}
