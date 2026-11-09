package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Level string
type Mode string
type SessionStatus string
type Difficulty string
type RecommendedAction string

const (
	LevelJunior Level = "junior"
	LevelMiddle Level = "middle"
	LevelSenior Level = "senior"

	ModeQuick    Mode = "quick"
	ModeStandard Mode = "standard"
	ModeFull     Mode = "full"

	StatusActive   SessionStatus = "active"
	StatusFinished SessionStatus = "finished"
	StatusStopped  SessionStatus = "stopped"

	DifficultyEasier Difficulty = "easier"
	DifficultySame   Difficulty = "same"
	DifficultyHarder Difficulty = "harder"

	ActionNextTopic RecommendedAction = "next_topic"
	ActionFollowUp  RecommendedAction = "follow_up"
	ActionFinish    RecommendedAction = "finish"
)

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Specialization struct {
	ID       int64
	Slug     string
	Name     string
	Category string
	Active   bool
}

type PlanArea struct {
	Topic  string `json:"topic"`
	Weight int    `json:"weight"`
}

type InterviewSession struct {
	ID                    int64
	UserID                int64
	SpecializationID      int64
	SpecializationSlug    string
	SpecializationName    string
	Level                 Level
	Mode                  Mode
	Status                SessionStatus
	StartedAt             time.Time
	FinishedAt            *time.Time
	OverallScore          *int
	Grade                 string
	InterviewLanguage     string
	CurrentQuestion       string
	CurrentTopic          string
	CurrentDifficulty     Difficulty
	QuestionCount         int
	FollowUpCountForTopic int
	Plan                  []PlanArea
	State                 InterviewState
}

type InterviewState struct {
	CoveredTopics map[string]int `json:"covered_topics"`
	StrongTopics  map[string]int `json:"strong_topics"`
	WeakTopics    map[string]int `json:"weak_topics"`
	CurrentTopic  string         `json:"current_topic"`
	Difficulty    Difficulty     `json:"difficulty"`
}

type InterviewAnswer struct {
	ID         int64
	SessionID  int64
	Question   string
	Answer     string
	Topic      string
	Difficulty Difficulty
	IsFollowUp bool
	Order      int
	Score      int
	Evaluation Evaluation
	CreatedAt  time.Time
}

type Evaluation struct {
	Score                 int               `json:"score"`
	Correctness           int               `json:"correctness"`
	Depth                 int               `json:"depth"`
	Clarity               int               `json:"clarity"`
	Strengths             []string          `json:"strengths"`
	Weaknesses            []string          `json:"weaknesses"`
	MissingPoints         []string          `json:"missing_points"`
	TopicsDetected        []string          `json:"topics_detected"`
	RecommendedAction     RecommendedAction `json:"recommended_action"`
	RecommendedDifficulty Difficulty        `json:"recommended_difficulty"`
	ReasoningSummary      string            `json:"reasoning_summary"`
}

type QuestionRequest struct {
	Specialization Specialization
	Level          Level
	Mode           Mode
	Language       string
	Plan           []PlanArea
	State          InterviewState
	History        []InterviewAnswer
	PreferredTopic string
	Action         RecommendedAction
	Difficulty     Difficulty
}

type GeneratedQuestion struct {
	Question   string     `json:"question"`
	Topic      string     `json:"topic"`
	Difficulty Difficulty `json:"difficulty"`
	IsFollowUp bool       `json:"is_follow_up"`
}

type Feedback struct {
	OverallScore    int            `json:"overall_score"`
	Grade           string         `json:"grade"`
	Verdict         string         `json:"verdict"`
	TopicScores     map[string]int `json:"topic_scores"`
	Strengths       []string       `json:"strengths"`
	Weaknesses      []string       `json:"weaknesses"`
	CriticalGaps    []string       `json:"critical_gaps"`
	StudyPlan       []string       `json:"study_plan"`
	Recommendations []string       `json:"recommendations"`
}

func (e *Evaluation) UnmarshalJSON(data []byte) error {
	type evaluationAlias Evaluation
	var raw struct {
		evaluationAlias
		Strengths      json.RawMessage `json:"strengths"`
		Weaknesses     json.RawMessage `json:"weaknesses"`
		MissingPoints  json.RawMessage `json:"missing_points"`
		TopicsDetected json.RawMessage `json:"topics_detected"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = Evaluation(raw.evaluationAlias)
	var err error
	if e.Strengths, err = stringList(raw.Strengths); err != nil {
		return fmt.Errorf("strengths: %w", err)
	}
	if e.Weaknesses, err = stringList(raw.Weaknesses); err != nil {
		return fmt.Errorf("weaknesses: %w", err)
	}
	if e.MissingPoints, err = stringList(raw.MissingPoints); err != nil {
		return fmt.Errorf("missing_points: %w", err)
	}
	if e.TopicsDetected, err = stringList(raw.TopicsDetected); err != nil {
		return fmt.Errorf("topics_detected: %w", err)
	}
	return nil
}

func (f *Feedback) UnmarshalJSON(data []byte) error {
	type feedbackAlias Feedback
	var raw struct {
		feedbackAlias
		Strengths       json.RawMessage `json:"strengths"`
		Weaknesses      json.RawMessage `json:"weaknesses"`
		CriticalGaps    json.RawMessage `json:"critical_gaps"`
		StudyPlan       json.RawMessage `json:"study_plan"`
		Recommendations json.RawMessage `json:"recommendations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*f = Feedback(raw.feedbackAlias)
	var err error
	if f.Strengths, err = stringList(raw.Strengths); err != nil {
		return fmt.Errorf("strengths: %w", err)
	}
	if f.Weaknesses, err = stringList(raw.Weaknesses); err != nil {
		return fmt.Errorf("weaknesses: %w", err)
	}
	if f.CriticalGaps, err = stringList(raw.CriticalGaps); err != nil {
		return fmt.Errorf("critical_gaps: %w", err)
	}
	if f.StudyPlan, err = stringList(raw.StudyPlan); err != nil {
		return fmt.Errorf("study_plan: %w", err)
	}
	if f.Recommendations, err = stringList(raw.Recommendations); err != nil {
		return fmt.Errorf("recommendations: %w", err)
	}
	return nil
}

func stringList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return compactStrings(list), nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}
	return nil, errors.New("expected string or string array")
}

func compactStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func TargetQuestions(mode Mode) int {
	switch mode {
	case ModeQuick:
		return 5
	case ModeFull:
		return 18
	default:
		return 10
	}
}

func ValidateLevel(level Level) error {
	switch level {
	case LevelJunior, LevelMiddle, LevelSenior:
		return nil
	default:
		return errors.New("invalid level")
	}
}

func ValidateMode(mode Mode) error {
	switch mode {
	case ModeQuick, ModeStandard, ModeFull:
		return nil
	default:
		return errors.New("invalid mode")
	}
}
