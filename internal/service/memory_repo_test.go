package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"ai-interview-coach/internal/domain"
)

type memoryRepo struct {
	users    map[int64]domain.User
	specs    map[string]domain.Specialization
	sessions map[int64]domain.InterviewSession
	answers  map[int64][]domain.InterviewAnswer
	nextSID  int64
	nextAID  int64
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		users: map[int64]domain.User{},
		specs: map[string]domain.Specialization{
			"go-backend": {ID: 1, Slug: "go-backend", Name: "Go Developer", Category: "backend", Active: true},
		},
		sessions: map[int64]domain.InterviewSession{},
		answers:  map[int64][]domain.InterviewAnswer{},
		nextSID:  1,
		nextAID:  1,
	}
}

func (m *memoryRepo) UpsertTelegramUser(ctx context.Context, telegramID int64, username string) (domain.User, error) {
	if u, ok := m.users[telegramID]; ok {
		u.Username = username
		m.users[telegramID] = u
		return u, nil
	}
	u := domain.User{ID: int64(len(m.users) + 1), TelegramID: telegramID, Username: username, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.users[telegramID] = u
	return u, nil
}

func (m *memoryRepo) ListSpecializations(ctx context.Context) ([]domain.Specialization, error) {
	out := make([]domain.Specialization, 0, len(m.specs))
	for _, s := range m.specs {
		out = append(out, s)
	}
	return out, nil
}

func (m *memoryRepo) GetSpecializationBySlug(ctx context.Context, slug string) (domain.Specialization, error) {
	s, ok := m.specs[slug]
	if !ok {
		return domain.Specialization{}, errors.New("not found")
	}
	return s, nil
}

func (m *memoryRepo) CreateSession(ctx context.Context, session domain.InterviewSession) (domain.InterviewSession, error) {
	session.ID = m.nextSID
	m.nextSID++
	session.StartedAt = time.Now()
	m.sessions[session.ID] = session
	return session, nil
}

func (m *memoryRepo) GetSession(ctx context.Context, sessionID int64) (domain.InterviewSession, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return domain.InterviewSession{}, errors.New("not found")
	}
	return s, nil
}

func (m *memoryRepo) GetActiveSessionByUser(ctx context.Context, userID int64) (domain.InterviewSession, error) {
	for _, s := range m.sessions {
		if s.UserID == userID && s.Status == domain.StatusActive {
			return s, nil
		}
	}
	return domain.InterviewSession{}, errors.New("not found")
}

func (m *memoryRepo) UpdateSession(ctx context.Context, session domain.InterviewSession) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *memoryRepo) SaveAnswer(ctx context.Context, answer domain.InterviewAnswer) (domain.InterviewAnswer, error) {
	answer.ID = m.nextAID
	m.nextAID++
	answer.CreatedAt = time.Now()
	m.answers[answer.SessionID] = append(m.answers[answer.SessionID], answer)
	return answer, nil
}

func (m *memoryRepo) ListAnswers(ctx context.Context, sessionID int64) ([]domain.InterviewAnswer, error) {
	out := append([]domain.InterviewAnswer{}, m.answers[sessionID]...)
	sort.Slice(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, nil
}

func (m *memoryRepo) ListSessionsByUser(ctx context.Context, userID int64, limit int) ([]domain.InterviewSession, error) {
	var out []domain.InterviewSession
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}
