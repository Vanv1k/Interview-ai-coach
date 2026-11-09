package interview

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"ai-interview-coach/internal/domain"
)

var (
	ErrEmptyAnswer     = errors.New("answer is empty")
	ErrAnswerTooLong   = errors.New("answer is too long")
	ErrSessionFinished = errors.New("session is finished")
)

type EngineConfig struct {
	MaxFollowUpsPerTopic int
	MaxAnswerLength      int
	MaxSameTopicStreak   int
}

type Engine struct {
	llm     LLMClient
	catalog Catalog
	cfg     EngineConfig
}

func NewEngine(llm LLMClient, catalog Catalog, cfg EngineConfig) *Engine {
	if cfg.MaxFollowUpsPerTopic <= 0 {
		cfg.MaxFollowUpsPerTopic = 2
	}
	if cfg.MaxAnswerLength <= 0 {
		cfg.MaxAnswerLength = 6000
	}
	if cfg.MaxSameTopicStreak <= 0 {
		cfg.MaxSameTopicStreak = 2
	}
	return &Engine{llm: llm, catalog: catalog, cfg: cfg}
}

func (e *Engine) NewSession(spec domain.Specialization, level domain.Level, mode domain.Mode) domain.InterviewSession {
	plan := e.catalog.Plan(spec.Slug, level)
	return domain.InterviewSession{
		SpecializationID:   spec.ID,
		SpecializationSlug: spec.Slug,
		SpecializationName: spec.Name,
		Level:              level,
		Mode:               mode,
		Status:             domain.StatusActive,
		Plan:               plan,
		InterviewLanguage:  "ru",
		CurrentDifficulty:  domain.DifficultySame,
		State: domain.InterviewState{
			CoveredTopics: map[string]int{},
			StrongTopics:  map[string]int{},
			WeakTopics:    map[string]int{},
			Difficulty:    domain.DifficultySame,
		},
	}
}

func (e *Engine) GenerateFirstQuestion(ctx context.Context, spec domain.Specialization, session domain.InterviewSession) (domain.GeneratedQuestion, error) {
	topic := e.pickTopic(session, nil)
	req := domain.QuestionRequest{
		Specialization: spec,
		Level:          session.Level,
		Mode:           session.Mode,
		Language:       languageOrDefault(session.InterviewLanguage),
		Plan:           session.Plan,
		State:          session.State,
		PreferredTopic: topic,
		Action:         domain.ActionNextTopic,
		Difficulty:     domain.DifficultySame,
	}
	return e.generateQuestion(ctx, spec.Slug, req)
}

type ProcessResult struct {
	Evaluation     domain.Evaluation
	SavedAnswer    domain.InterviewAnswer
	NextQuestion   *domain.GeneratedQuestion
	FinalFeedback  *domain.Feedback
	UpdatedSession domain.InterviewSession
}

func (e *Engine) ProcessAnswer(ctx context.Context, spec domain.Specialization, session domain.InterviewSession, history []domain.InterviewAnswer, answer string) (ProcessResult, error) {
	if session.Status != domain.StatusActive {
		return ProcessResult{}, ErrSessionFinished
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ProcessResult{}, ErrEmptyAnswer
	}
	if len([]rune(answer)) > e.cfg.MaxAnswerLength {
		return ProcessResult{}, ErrAnswerTooLong
	}

	eval, err := e.llm.EvaluateAnswer(ctx, e.systemPrompt(spec.Slug, session.InterviewLanguage), session, history, answer)
	if err != nil {
		return ProcessResult{}, err
	}
	if err := validateEvaluation(eval); err != nil {
		return ProcessResult{}, err
	}

	record := domain.InterviewAnswer{
		SessionID:  session.ID,
		Question:   session.CurrentQuestion,
		Answer:     answer,
		Topic:      session.CurrentTopic,
		Difficulty: session.CurrentDifficulty,
		IsFollowUp: session.FollowUpCountForTopic > 0,
		Order:      session.QuestionCount + 1,
		Score:      eval.Score,
		Evaluation: eval,
	}

	nextHistory := append(append([]domain.InterviewAnswer{}, history...), record)
	session.QuestionCount++
	session.State = updateState(session.State, record)
	session.CurrentDifficulty = eval.RecommendedDifficulty
	if session.CurrentDifficulty == "" {
		session.CurrentDifficulty = domain.DifficultySame
	}

	if shouldFinish(session, nextHistory) || eval.RecommendedAction == domain.ActionFinish {
		aggregate := AggregateFeedback(session, nextHistory)
		feedback, err := e.llm.GenerateFinalFeedback(ctx, e.systemPrompt(spec.Slug, session.InterviewLanguage), session, nextHistory, aggregate)
		if err != nil {
			return ProcessResult{}, err
		}
		feedback.OverallScore = aggregate.OverallScore
		feedback.Grade = aggregate.Grade
		if len(feedback.TopicScores) == 0 {
			feedback.TopicScores = aggregate.TopicScores
		}
		session.Status = domain.StatusFinished
		session.OverallScore = &feedback.OverallScore
		session.Grade = feedback.Grade
		return ProcessResult{Evaluation: eval, SavedAnswer: record, FinalFeedback: &feedback, UpdatedSession: session}, nil
	}

	action := normalizeAction(eval.RecommendedAction)
	if action == domain.ActionFollowUp && session.FollowUpCountForTopic >= e.cfg.MaxFollowUpsPerTopic {
		action = domain.ActionNextTopic
	}
	preferred := session.CurrentTopic
	if action == domain.ActionNextTopic {
		preferred = e.pickTopicExcluding(session, nextHistory, session.CurrentTopic)
		session.FollowUpCountForTopic = 0
	} else {
		session.FollowUpCountForTopic++
	}

	req := domain.QuestionRequest{
		Specialization: spec,
		Level:          session.Level,
		Mode:           session.Mode,
		Language:       languageOrDefault(session.InterviewLanguage),
		Plan:           session.Plan,
		State:          session.State,
		History:        nextHistory,
		PreferredTopic: preferred,
		Action:         action,
		Difficulty:     session.CurrentDifficulty,
	}
	next, err := e.generateQuestion(ctx, spec.Slug, req)
	if err != nil {
		return ProcessResult{}, err
	}
	if e.shouldReplaceRepeatedQuestion(next, session, nextHistory) {
		next = e.fallbackQuestion(session, nextHistory)
		session.FollowUpCountForTopic = 0
	}
	session.CurrentQuestion = next.Question
	session.CurrentTopic = next.Topic
	session.CurrentDifficulty = next.Difficulty
	session.State.CurrentTopic = next.Topic
	session.State.Difficulty = next.Difficulty

	return ProcessResult{Evaluation: eval, SavedAnswer: record, NextQuestion: &next, UpdatedSession: session}, nil
}

func (e *Engine) generateQuestion(ctx context.Context, specSlug string, req domain.QuestionRequest) (domain.GeneratedQuestion, error) {
	q, err := e.llm.GenerateQuestion(ctx, e.systemPrompt(specSlug, req.Language), req)
	if err != nil {
		return domain.GeneratedQuestion{}, err
	}
	if strings.TrimSpace(q.Question) == "" {
		return domain.GeneratedQuestion{}, errors.New("llm returned empty question")
	}
	if q.Topic == "" {
		q.Topic = req.PreferredTopic
	}
	if q.Difficulty == "" {
		q.Difficulty = req.Difficulty
	}
	if q.Difficulty == "" {
		q.Difficulty = domain.DifficultySame
	}
	q.IsFollowUp = req.Action == domain.ActionFollowUp
	return q, nil
}

func (e *Engine) systemPrompt(slug string, language string) string {
	return "You are conducting a realistic technical interview. Do not teach the candidate during the interview. Ask concise interview questions. Evaluate answers critically but fairly. Ask all interview questions and write final feedback in " + languageName(language) + ". " + e.catalog.Prompt(slug)
}

func languageOrDefault(language string) string {
	if strings.TrimSpace(language) == "" {
		return "ru"
	}
	return language
}

func languageName(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ru", "russian":
		return "Russian"
	case "en", "english":
		return "English"
	default:
		return language
	}
}

func (e *Engine) pickTopic(session domain.InterviewSession, history []domain.InterviewAnswer) string {
	return e.pickTopicExcluding(session, history, "")
}

func (e *Engine) pickTopicExcluding(session domain.InterviewSession, history []domain.InterviewAnswer, excludedTopic string) string {
	counts := map[string]int{}
	for _, ans := range history {
		counts[ans.Topic]++
	}
	var selected string
	best := int(^uint(0) >> 1)
	for _, area := range session.Plan {
		if excludedTopic != "" && area.Topic == excludedTopic && len(session.Plan) > 1 {
			continue
		}
		expected := max(1, domain.TargetQuestions(session.Mode)*area.Weight/100)
		ratio := counts[area.Topic] * 100 / expected
		if ratio < best {
			best = ratio
			selected = area.Topic
		}
	}
	if selected == "" && len(session.Plan) > 0 {
		selected = session.Plan[0].Topic
	}
	return selected
}

func (e *Engine) shouldReplaceRepeatedQuestion(next domain.GeneratedQuestion, session domain.InterviewSession, history []domain.InterviewAnswer) bool {
	if next.Question == "" {
		return false
	}
	if similarQuestions(next.Question, session.CurrentQuestion) {
		return true
	}
	if next.Topic == session.CurrentTopic && sameTopicStreak(history, session.CurrentTopic) >= e.cfg.MaxSameTopicStreak {
		return true
	}
	for i := len(history) - 1; i >= 0 && i >= len(history)-3; i-- {
		if similarQuestions(next.Question, history[i].Question) {
			return true
		}
	}
	return false
}

func (e *Engine) fallbackQuestion(session domain.InterviewSession, history []domain.InterviewAnswer) domain.GeneratedQuestion {
	topic := e.pickTopicExcluding(session, history, session.CurrentTopic)
	if topic == "" {
		topic = e.pickTopic(session, history)
	}
	return domain.GeneratedQuestion{
		Question:   fallbackQuestionText(topic, languageOrDefault(session.InterviewLanguage)),
		Topic:      topic,
		Difficulty: domain.DifficultySame,
		IsFollowUp: false,
	}
}

func sameTopicStreak(history []domain.InterviewAnswer, topic string) int {
	count := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Topic != topic {
			break
		}
		count++
	}
	return count
}

func fallbackQuestionText(topic string, language string) string {
	if strings.EqualFold(language, "en") {
		return "Let's switch topics. Describe a practical production issue related to " + topic + " and how you would diagnose it."
	}
	return "Перейдём к другой теме. Опишите практическую production-проблему по теме " + topic + " и как бы вы её диагностировали."
}

var wordRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

func similarQuestions(a, b string) bool {
	aWords := significantWords(a)
	bWords := significantWords(b)
	if len(aWords) == 0 || len(bWords) == 0 {
		return false
	}
	common := 0
	for word := range aWords {
		if bWords[word] {
			common++
		}
	}
	smaller := min(len(aWords), len(bWords))
	return float64(common)/float64(smaller) >= 0.65
}

func significantWords(text string) map[string]bool {
	words := wordRE.FindAllString(strings.ToLower(text), -1)
	out := make(map[string]bool, len(words))
	for _, word := range words {
		if len([]rune(word)) < 4 || stopWords[word] {
			continue
		}
		out[word] = true
	}
	return out
}

var stopWords = map[string]bool{
	"какие": true, "какой": true, "какая": true, "какое": true, "чтобы": true, "такое": true,
	"объясните": true, "опишите": true, "расскажите": true, "работе": true, "работает": true,
	"describe": true, "explain": true, "what": true, "which": true, "that": true, "with": true,
}

func shouldFinish(session domain.InterviewSession, answers []domain.InterviewAnswer) bool {
	return session.QuestionCount >= domain.TargetQuestions(session.Mode) || len(answers) >= domain.TargetQuestions(session.Mode)
}

func updateState(state domain.InterviewState, answer domain.InterviewAnswer) domain.InterviewState {
	if state.CoveredTopics == nil {
		state.CoveredTopics = map[string]int{}
	}
	if state.StrongTopics == nil {
		state.StrongTopics = map[string]int{}
	}
	if state.WeakTopics == nil {
		state.WeakTopics = map[string]int{}
	}
	state.CoveredTopics[answer.Topic]++
	if answer.Score >= 8 {
		state.StrongTopics[answer.Topic]++
	}
	if answer.Score <= 5 {
		state.WeakTopics[answer.Topic]++
	}
	state.CurrentTopic = answer.Topic
	state.Difficulty = answer.Evaluation.RecommendedDifficulty
	return state
}

func validateEvaluation(eval domain.Evaluation) error {
	if eval.Score < 0 || eval.Score > 10 || eval.Correctness < 0 || eval.Correctness > 10 || eval.Depth < 0 || eval.Depth > 10 || eval.Clarity < 0 || eval.Clarity > 10 {
		return fmt.Errorf("invalid evaluation scores")
	}
	switch normalizeAction(eval.RecommendedAction) {
	case domain.ActionFollowUp, domain.ActionNextTopic, domain.ActionFinish:
	default:
		return fmt.Errorf("invalid recommended action")
	}
	switch eval.RecommendedDifficulty {
	case "", domain.DifficultyEasier, domain.DifficultySame, domain.DifficultyHarder:
	default:
		return fmt.Errorf("invalid recommended difficulty")
	}
	return nil
}

func normalizeAction(action domain.RecommendedAction) domain.RecommendedAction {
	switch action {
	case domain.ActionFollowUp, domain.ActionFinish:
		return action
	default:
		return domain.ActionNextTopic
	}
}
