package interview

import (
	"context"
	"errors"
	"testing"

	"ai-interview-coach/internal/domain"
)

type fakeLLM struct {
	evals     []domain.Evaluation
	questions []domain.GeneratedQuestion
	feedback  domain.Feedback
}

func (f *fakeLLM) GenerateQuestion(ctx context.Context, systemPrompt string, req domain.QuestionRequest) (domain.GeneratedQuestion, error) {
	if len(f.questions) == 0 {
		return domain.GeneratedQuestion{}, errors.New("no question")
	}
	q := f.questions[0]
	f.questions = f.questions[1:]
	return q, nil
}

func (f *fakeLLM) EvaluateAnswer(ctx context.Context, systemPrompt string, session domain.InterviewSession, history []domain.InterviewAnswer, answer string) (domain.Evaluation, error) {
	if len(f.evals) == 0 {
		return domain.Evaluation{}, errors.New("no eval")
	}
	e := f.evals[0]
	f.evals = f.evals[1:]
	return e, nil
}

func (f *fakeLLM) GenerateFinalFeedback(ctx context.Context, systemPrompt string, session domain.InterviewSession, answers []domain.InterviewAnswer, aggregate domain.Feedback) (domain.Feedback, error) {
	f.feedback.OverallScore = aggregate.OverallScore
	f.feedback.Grade = aggregate.Grade
	f.feedback.TopicScores = aggregate.TopicScores
	return f.feedback, nil
}

func TestEngineFollowUpThenStopsAtLimit(t *testing.T) {
	llm := &fakeLLM{
		evals: []domain.Evaluation{
			{Score: 4, Correctness: 4, Depth: 3, Clarity: 5, RecommendedAction: domain.ActionFollowUp, RecommendedDifficulty: domain.DifficultyEasier},
			{Score: 5, Correctness: 5, Depth: 4, Clarity: 6, RecommendedAction: domain.ActionFollowUp, RecommendedDifficulty: domain.DifficultySame},
		},
		questions: []domain.GeneratedQuestion{
			{Question: "follow-up?", Topic: "Concurrency", Difficulty: domain.DifficultyEasier},
			{Question: "next topic?", Topic: "Go Core", Difficulty: domain.DifficultySame},
		},
	}
	engine := NewEngine(llm, DefaultCatalog(), EngineConfig{MaxFollowUpsPerTopic: 1, MaxAnswerLength: 1000})
	spec := domain.Specialization{ID: 1, Slug: "go-backend", Name: "Go Developer"}
	session := engine.NewSession(spec, domain.LevelMiddle, domain.ModeStandard)
	session.ID = 10
	session.CurrentQuestion = "q1"
	session.CurrentTopic = "Concurrency"
	session.CurrentDifficulty = domain.DifficultySame

	res, err := engine.ProcessAnswer(context.Background(), spec, session, nil, "shallow answer")
	if err != nil {
		t.Fatal(err)
	}
	if res.NextQuestion == nil || !res.NextQuestion.IsFollowUp {
		t.Fatalf("expected follow-up question, got %#v", res.NextQuestion)
	}

	session = res.UpdatedSession
	res, err = engine.ProcessAnswer(context.Background(), spec, session, []domain.InterviewAnswer{res.SavedAnswer}, "still shallow")
	if err != nil {
		t.Fatal(err)
	}
	if res.NextQuestion == nil || res.NextQuestion.Topic != "Go Core" {
		t.Fatalf("expected switch to next topic after follow-up limit, got %#v", res.NextQuestion)
	}
}

func TestEngineRejectsMalformedEvaluation(t *testing.T) {
	llm := &fakeLLM{evals: []domain.Evaluation{{Score: 99, Correctness: 1, Depth: 1, Clarity: 1, RecommendedAction: domain.ActionNextTopic}}}
	engine := NewEngine(llm, DefaultCatalog(), EngineConfig{MaxAnswerLength: 1000})
	spec := domain.Specialization{ID: 1, Slug: "go-backend"}
	session := engine.NewSession(spec, domain.LevelMiddle, domain.ModeQuick)
	session.ID = 1
	session.CurrentQuestion = "q"
	session.CurrentTopic = "Go Core"
	_, err := engine.ProcessAnswer(context.Background(), spec, session, nil, "answer")
	if err == nil {
		t.Fatal("expected malformed evaluation error")
	}
}

func TestEngineFinishesQuickInterviewWithBackendGrade(t *testing.T) {
	llm := &fakeLLM{
		evals: []domain.Evaluation{{Score: 8, Correctness: 8, Depth: 8, Clarity: 8, RecommendedAction: domain.ActionNextTopic, RecommendedDifficulty: domain.DifficultySame}},
		feedback: domain.Feedback{
			Verdict: "good",
		},
	}
	engine := NewEngine(llm, DefaultCatalog(), EngineConfig{MaxAnswerLength: 1000})
	spec := domain.Specialization{ID: 1, Slug: "go-backend"}
	session := engine.NewSession(spec, domain.LevelMiddle, domain.ModeQuick)
	session.ID = 1
	session.QuestionCount = domain.TargetQuestions(domain.ModeQuick) - 1
	session.CurrentQuestion = "q"
	session.CurrentTopic = "Go Core"
	res, err := engine.ProcessAnswer(context.Background(), spec, session, nil, "answer")
	if err != nil {
		t.Fatal(err)
	}
	if res.FinalFeedback == nil {
		t.Fatal("expected final feedback")
	}
	if res.FinalFeedback.OverallScore == 0 || res.FinalFeedback.Grade == "" {
		t.Fatalf("backend aggregate should populate score and grade: %#v", res.FinalFeedback)
	}
}

func TestEngineReplacesRepeatedQuestionWithDifferentTopic(t *testing.T) {
	llm := &fakeLLM{
		evals: []domain.Evaluation{{Score: 1, Correctness: 1, Depth: 1, Clarity: 1, RecommendedAction: domain.ActionFollowUp, RecommendedDifficulty: domain.DifficultySame}},
		questions: []domain.GeneratedQuestion{
			{Question: "Опишите, что такое модель памяти в Go и какие гарантии она предоставляет при работе с конкурентными горутинами.", Topic: "Go Core", Difficulty: domain.DifficultySame},
		},
	}
	engine := NewEngine(llm, DefaultCatalog(), EngineConfig{MaxFollowUpsPerTopic: 2, MaxSameTopicStreak: 2, MaxAnswerLength: 1000})
	spec := domain.Specialization{ID: 1, Slug: "go-backend"}
	session := engine.NewSession(spec, domain.LevelMiddle, domain.ModeStandard)
	session.ID = 1
	session.CurrentQuestion = "Объясните, что такое модель памяти в Go и какие гарантии она предоставляет при работе с конкурентными горутинами."
	session.CurrentTopic = "Go Core"
	session.CurrentDifficulty = domain.DifficultySame
	session.FollowUpCountForTopic = 1

	history := []domain.InterviewAnswer{
		{Question: "Объясните, как работает сборщик мусора в Go.", Topic: "Go Core", Evaluation: domain.Evaluation{Score: 1}},
		{Question: session.CurrentQuestion, Topic: "Go Core", Evaluation: domain.Evaluation{Score: 1}},
	}
	res, err := engine.ProcessAnswer(context.Background(), spec, session, history, "не знаю")
	if err != nil {
		t.Fatal(err)
	}
	if res.NextQuestion == nil {
		t.Fatal("expected next question")
	}
	if res.NextQuestion.Topic == "Go Core" {
		t.Fatalf("expected fallback to another topic, got %#v", res.NextQuestion)
	}
	if res.NextQuestion.IsFollowUp {
		t.Fatalf("fallback should not be a follow-up: %#v", res.NextQuestion)
	}
}
