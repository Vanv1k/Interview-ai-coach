package domain

import (
	"encoding/json"
	"testing"
)

func TestEvaluationUnmarshalAcceptsStringLists(t *testing.T) {
	raw := []byte(`{
		"score": 7,
		"correctness": 8,
		"depth": 6,
		"clarity": 7,
		"strengths": "корректно описал неявную реализацию интерфейса",
		"weaknesses": ["не сравнил с Java подробно"],
		"missing_points": "",
		"topics_detected": "interfaces",
		"recommended_action": "next_topic",
		"recommended_difficulty": "same",
		"reasoning_summary": "ok"
	}`)
	var eval Evaluation
	if err := json.Unmarshal(raw, &eval); err != nil {
		t.Fatal(err)
	}
	if len(eval.Strengths) != 1 || eval.Strengths[0] == "" {
		t.Fatalf("unexpected strengths: %#v", eval.Strengths)
	}
	if len(eval.MissingPoints) != 0 {
		t.Fatalf("empty string should be omitted: %#v", eval.MissingPoints)
	}
	if len(eval.TopicsDetected) != 1 || eval.TopicsDetected[0] != "interfaces" {
		t.Fatalf("unexpected topics: %#v", eval.TopicsDetected)
	}
}

func TestFeedbackUnmarshalAcceptsStringLists(t *testing.T) {
	raw := []byte(`{
		"overall_score": 72,
		"grade": "Middle",
		"verdict": "ok",
		"topic_scores": {"Go Core": 80},
		"strengths": "хорошая база",
		"weaknesses": "мало деталей",
		"critical_gaps": null,
		"study_plan": ["channels"],
		"recommendations": ""
	}`)
	var feedback Feedback
	if err := json.Unmarshal(raw, &feedback); err != nil {
		t.Fatal(err)
	}
	if len(feedback.Strengths) != 1 || len(feedback.Weaknesses) != 1 {
		t.Fatalf("unexpected feedback lists: %#v %#v", feedback.Strengths, feedback.Weaknesses)
	}
	if len(feedback.Recommendations) != 0 {
		t.Fatalf("empty recommendation should be omitted: %#v", feedback.Recommendations)
	}
}
