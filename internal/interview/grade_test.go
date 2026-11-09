package interview

import (
	"testing"

	"ai-interview-coach/internal/domain"
)

func TestGradeFromScore(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{20, "Junior-"},
		{40, "Junior"},
		{50, "Junior+"},
		{60, "Middle-"},
		{70, "Middle"},
		{80, "Middle+"},
		{86, "Senior-"},
		{92, "Senior"},
		{98, "Senior+"},
	}
	for _, tt := range tests {
		if got := GradeFromScore(tt.score); got != tt.want {
			t.Fatalf("GradeFromScore(%d)=%s want %s", tt.score, got, tt.want)
		}
	}
}

func TestAggregateFeedbackUsesBackendScoresAndCoverage(t *testing.T) {
	session := domain.InterviewSession{
		Mode: domain.ModeQuick,
		Plan: []domain.PlanArea{{Topic: "Go Core", Weight: 50}, {Topic: "Concurrency", Weight: 50}},
	}
	answers := []domain.InterviewAnswer{
		{Topic: "Go Core", Difficulty: domain.DifficultySame, Evaluation: domain.Evaluation{Correctness: 8, Depth: 7, Clarity: 9}},
		{Topic: "Concurrency", Difficulty: domain.DifficultyHarder, Evaluation: domain.Evaluation{Correctness: 6, Depth: 5, Clarity: 7}},
	}
	got := AggregateFeedback(session, answers)
	if got.OverallScore <= 0 || got.OverallScore > 100 {
		t.Fatalf("invalid overall score: %d", got.OverallScore)
	}
	if got.Grade == "" {
		t.Fatal("grade should be calculated")
	}
	if len(got.TopicScores) != 2 {
		t.Fatalf("expected topic scores for 2 topics, got %d", len(got.TopicScores))
	}
}
