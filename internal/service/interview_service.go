package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"ai-interview-coach/internal/domain"
	"ai-interview-coach/internal/interview"
	"ai-interview-coach/internal/ratelimit"
)

var (
	ErrNoActiveSession = errors.New("no active interview session")
	ErrRateLimited     = errors.New("rate limited")
)

type InterviewService struct {
	repo             domain.Repository
	engine           *interview.Engine
	logger           *slog.Logger
	limiter          ratelimit.Limiter
	answerRatePerMin int
	startRatePerHour int
}

func NewInterviewService(repo domain.Repository, engine *interview.Engine, logger *slog.Logger) *InterviewService {
	return &InterviewService{repo: repo, engine: engine, logger: logger}
}

type Limits struct {
	AnswerPerMinute int
	StartPerHour    int
}

func NewInterviewServiceWithLimiter(repo domain.Repository, engine *interview.Engine, logger *slog.Logger, limiter ratelimit.Limiter, limits Limits) *InterviewService {
	return &InterviewService{
		repo:             repo,
		engine:           engine,
		logger:           logger,
		limiter:          limiter,
		answerRatePerMin: limits.AnswerPerMinute,
		startRatePerHour: limits.StartPerHour,
	}
}

type StartInput struct {
	TelegramID         int64
	Username           string
	SpecializationSlug string
	Level              domain.Level
	Mode               domain.Mode
	LanguageCode       string
}

type StartResult struct {
	Session  domain.InterviewSession
	Question domain.GeneratedQuestion
}

func (s *InterviewService) StartInterview(ctx context.Context, in StartInput) (StartResult, error) {
	if !s.allow("start:"+strconv.FormatInt(in.TelegramID, 10), s.startRatePerHour, time.Hour) {
		return StartResult{}, ErrRateLimited
	}
	if err := domain.ValidateLevel(in.Level); err != nil {
		return StartResult{}, err
	}
	if err := domain.ValidateMode(in.Mode); err != nil {
		return StartResult{}, err
	}

	user, err := s.repo.UpsertTelegramUser(ctx, in.TelegramID, in.Username)
	if err != nil {
		return StartResult{}, err
	}
	spec, err := s.repo.GetSpecializationBySlug(ctx, in.SpecializationSlug)
	if err != nil {
		return StartResult{}, err
	}
	session := s.engine.NewSession(spec, in.Level, in.Mode)
	session.UserID = user.ID
	session.InterviewLanguage = normalizeLanguage(in.LanguageCode)
	question, err := s.engine.GenerateFirstQuestion(ctx, spec, session)
	if err != nil {
		return StartResult{}, err
	}
	session.CurrentQuestion = question.Question
	session.CurrentTopic = question.Topic
	session.CurrentDifficulty = question.Difficulty
	session.State.CurrentTopic = question.Topic
	session.State.Difficulty = question.Difficulty

	created, err := s.repo.CreateSession(ctx, session)
	if err != nil {
		return StartResult{}, err
	}
	s.logger.Info("session started", "session_id", created.ID, "user_id", user.ID, "specialization", spec.Slug, "level", in.Level, "mode", in.Mode)
	return StartResult{Session: created, Question: question}, nil
}

type AnswerResult struct {
	Session       domain.InterviewSession
	Evaluation    domain.Evaluation
	NextQuestion  *domain.GeneratedQuestion
	FinalFeedback *domain.Feedback
}

func (s *InterviewService) SubmitAnswer(ctx context.Context, telegramID int64, username string, answer string) (AnswerResult, error) {
	if !s.allow("answer:"+strconv.FormatInt(telegramID, 10), s.answerRatePerMin, time.Minute) {
		return AnswerResult{}, ErrRateLimited
	}
	user, err := s.repo.UpsertTelegramUser(ctx, telegramID, username)
	if err != nil {
		return AnswerResult{}, err
	}
	session, err := s.repo.GetActiveSessionByUser(ctx, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AnswerResult{}, ErrNoActiveSession
		}
		return AnswerResult{}, err
	}
	spec, err := s.repo.GetSpecializationBySlug(ctx, session.SpecializationSlug)
	if err != nil {
		return AnswerResult{}, err
	}
	history, err := s.repo.ListAnswers(ctx, session.ID)
	if err != nil {
		return AnswerResult{}, err
	}

	result, err := s.engine.ProcessAnswer(ctx, spec, session, history, answer)
	if err != nil {
		return AnswerResult{}, err
	}
	if _, err := s.repo.SaveAnswer(ctx, result.SavedAnswer); err != nil {
		return AnswerResult{}, err
	}
	updated := result.UpdatedSession
	if updated.Status == domain.StatusFinished {
		now := time.Now().UTC()
		updated.FinishedAt = &now
	}
	if err := s.repo.UpdateSession(ctx, updated); err != nil {
		return AnswerResult{}, err
	}
	if updated.Status == domain.StatusFinished {
		s.logger.Info("session finished", "session_id", updated.ID, "score", derefInt(updated.OverallScore), "grade", updated.Grade)
	}
	return AnswerResult{Session: updated, Evaluation: result.Evaluation, NextQuestion: result.NextQuestion, FinalFeedback: result.FinalFeedback}, nil
}

func (s *InterviewService) StopActive(ctx context.Context, telegramID int64, username string) error {
	user, err := s.repo.UpsertTelegramUser(ctx, telegramID, username)
	if err != nil {
		return err
	}
	session, err := s.repo.GetActiveSessionByUser(ctx, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ErrNoActiveSession
		}
		return err
	}
	now := time.Now().UTC()
	session.Status = domain.StatusStopped
	session.FinishedAt = &now
	return s.repo.UpdateSession(ctx, session)
}

func (s *InterviewService) History(ctx context.Context, telegramID int64, username string, limit int) ([]domain.InterviewSession, error) {
	user, err := s.repo.UpsertTelegramUser(ctx, telegramID, username)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	return s.repo.ListSessionsByUser(ctx, user.ID, limit)
}

func FormatFeedback(f domain.Feedback) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Итоговый score: %d/100\nGrade: %s\n\n", f.OverallScore, f.Grade)
	if f.Verdict != "" {
		fmt.Fprintf(&b, "Вердикт: %s\n\n", f.Verdict)
	}
	if len(f.TopicScores) > 0 {
		b.WriteString("Разбивка по темам:\n")
		for topic, score := range f.TopicScores {
			fmt.Fprintf(&b, "- %s: %d/100\n", topic, score)
		}
		b.WriteString("\n")
	}
	writeList(&b, "Сильные стороны", f.Strengths)
	writeList(&b, "Слабые стороны", f.Weaknesses)
	writeList(&b, "Critical gaps", f.CriticalGaps)
	writeList(&b, "Что повторить", f.StudyPlan)
	writeList(&b, "Рекомендации", f.Recommendations)
	return strings.TrimSpace(b.String())
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for i, item := range items {
		fmt.Fprintf(b, "%d. %s\n", i+1, item)
	}
	b.WriteString("\n")
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func normalizeLanguage(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if strings.HasPrefix(code, "en") {
		return "en"
	}
	return "ru"
}

func (s *InterviewService) allow(key string, limit int, window time.Duration) bool {
	if s.limiter == nil {
		return true
	}
	return s.limiter.Allow(key, limit, window)
}
