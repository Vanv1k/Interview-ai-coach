package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"ai-interview-coach/internal/domain"
	"ai-interview-coach/internal/interview"
	"ai-interview-coach/internal/ratelimit"
)

func TestServiceStartAndSubmitPersistsState(t *testing.T) {
	repo := newMemoryRepo()
	llm := &serviceLLM{
		questions: []domain.GeneratedQuestion{
			{Question: "What is a goroutine?", Topic: "Go Core", Difficulty: domain.DifficultySame},
			{Question: "What is a channel?", Topic: "Concurrency", Difficulty: domain.DifficultySame},
		},
		evals: []domain.Evaluation{{Score: 8, Correctness: 8, Depth: 7, Clarity: 8, RecommendedAction: domain.ActionNextTopic, RecommendedDifficulty: domain.DifficultySame}},
	}
	engine := interview.NewEngine(llm, interview.DefaultCatalog(), interview.EngineConfig{MaxAnswerLength: 1000})
	svc := NewInterviewService(repo, engine, slog.Default())

	start, err := svc.StartInterview(context.Background(), StartInput{TelegramID: 100, Username: "u", SpecializationSlug: "go-backend", Level: domain.LevelMiddle, Mode: domain.ModeStandard})
	if err != nil {
		t.Fatal(err)
	}
	if start.Session.ID == 0 || start.Question.Question == "" {
		t.Fatalf("invalid start result: %#v", start)
	}
	answer, err := svc.SubmitAnswer(context.Background(), 100, "u", "A goroutine is a lightweight concurrent function.")
	if err != nil {
		t.Fatal(err)
	}
	if answer.NextQuestion == nil {
		t.Fatal("expected next question")
	}
	saved, _ := repo.ListAnswers(context.Background(), start.Session.ID)
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved answer, got %d", len(saved))
	}
	active, err := repo.GetActiveSessionByUser(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if active.CurrentQuestion != "What is a channel?" {
		t.Fatalf("session state not updated, got %q", active.CurrentQuestion)
	}
}

func TestServiceRateLimitsAnswers(t *testing.T) {
	repo := newMemoryRepo()
	llm := &serviceLLM{
		questions: []domain.GeneratedQuestion{
			{Question: "q1", Topic: "Go Core", Difficulty: domain.DifficultySame},
			{Question: "q2", Topic: "Concurrency", Difficulty: domain.DifficultySame},
		},
		evals: []domain.Evaluation{{Score: 7, Correctness: 7, Depth: 7, Clarity: 7, RecommendedAction: domain.ActionNextTopic, RecommendedDifficulty: domain.DifficultySame}},
	}
	engine := interview.NewEngine(llm, interview.DefaultCatalog(), interview.EngineConfig{MaxAnswerLength: 1000})
	svc := NewInterviewServiceWithLimiter(repo, engine, slog.Default(), ratelimit.NewMemory(), Limits{AnswerPerMinute: 1, StartPerHour: 10})

	_, err := svc.StartInterview(context.Background(), StartInput{TelegramID: 100, Username: "u", SpecializationSlug: "go-backend", Level: domain.LevelMiddle, Mode: domain.ModeStandard})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitAnswer(context.Background(), 100, "u", "first answer"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.SubmitAnswer(context.Background(), 100, "u", "second answer")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limited error, got %v", err)
	}
}

type serviceLLM struct {
	questions []domain.GeneratedQuestion
	evals     []domain.Evaluation
}

func (f *serviceLLM) GenerateQuestion(ctx context.Context, systemPrompt string, req domain.QuestionRequest) (domain.GeneratedQuestion, error) {
	q := f.questions[0]
	f.questions = f.questions[1:]
	return q, nil
}

func (f *serviceLLM) EvaluateAnswer(ctx context.Context, systemPrompt string, session domain.InterviewSession, history []domain.InterviewAnswer, answer string) (domain.Evaluation, error) {
	e := f.evals[0]
	f.evals = f.evals[1:]
	return e, nil
}

func (f *serviceLLM) GenerateFinalFeedback(ctx context.Context, systemPrompt string, session domain.InterviewSession, answers []domain.InterviewAnswer, aggregate domain.Feedback) (domain.Feedback, error) {
	return aggregate, nil
}
