package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"ai-interview-coach/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertTelegramUser(ctx context.Context, telegramID int64, username string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		insert into users (telegram_id, username)
		values ($1, $2)
		on conflict (telegram_id) do update set username = excluded.username, updated_at = now()
		returning id, telegram_id, username, created_at, updated_at
	`, telegramID, username).Scan(&u.ID, &u.TelegramID, &u.Username, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *Repository) ListSpecializations(ctx context.Context) ([]domain.Specialization, error) {
	rows, err := r.db.Query(ctx, `select id, slug, name, category, active from specializations where active order by category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Specialization
	for rows.Next() {
		var s domain.Specialization
		if err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.Category, &s.Active); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *Repository) GetSpecializationBySlug(ctx context.Context, slug string) (domain.Specialization, error) {
	var s domain.Specialization
	err := r.db.QueryRow(ctx, `select id, slug, name, category, active from specializations where slug = $1 and active`, slug).
		Scan(&s.ID, &s.Slug, &s.Name, &s.Category, &s.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Specialization{}, domain.ErrNotFound
	}
	return s, err
}

func (r *Repository) CreateSession(ctx context.Context, session domain.InterviewSession) (domain.InterviewSession, error) {
	plan, state, err := marshalPlanState(session)
	if err != nil {
		return domain.InterviewSession{}, err
	}
	err = r.db.QueryRow(ctx, `
		insert into interview_sessions (
			user_id, specialization_id, level, mode, status, interview_language, current_question, current_topic,
			current_difficulty, question_count, follow_up_count_for_topic, plan_json, state_json
		)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		returning id, started_at
	`, session.UserID, session.SpecializationID, session.Level, session.Mode, session.Status,
		session.InterviewLanguage, session.CurrentQuestion, session.CurrentTopic, session.CurrentDifficulty, session.QuestionCount,
		session.FollowUpCountForTopic, plan, state).Scan(&session.ID, &session.StartedAt)
	return session, err
}

func (r *Repository) GetSession(ctx context.Context, sessionID int64) (domain.InterviewSession, error) {
	return r.scanSession(r.db.QueryRow(ctx, sessionQuery()+` where s.id = $1`, sessionID))
}

func (r *Repository) GetActiveSessionByUser(ctx context.Context, userID int64) (domain.InterviewSession, error) {
	session, err := r.scanSession(r.db.QueryRow(ctx, sessionQuery()+` where s.user_id = $1 and s.status = 'active' order by s.started_at desc limit 1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InterviewSession{}, domain.ErrNotFound
	}
	return session, err
}

func (r *Repository) UpdateSession(ctx context.Context, session domain.InterviewSession) error {
	plan, state, err := marshalPlanState(session)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		update interview_sessions set
			status=$2, finished_at=$3, overall_score=$4, grade=$5, interview_language=$6, current_question=$7,
			current_topic=$8, current_difficulty=$9, question_count=$10,
			follow_up_count_for_topic=$11, plan_json=$12, state_json=$13, updated_at=now()
		where id=$1
	`, session.ID, session.Status, session.FinishedAt, session.OverallScore, session.Grade,
		session.InterviewLanguage, session.CurrentQuestion, session.CurrentTopic, session.CurrentDifficulty, session.QuestionCount,
		session.FollowUpCountForTopic, plan, state)
	return err
}

func (r *Repository) SaveAnswer(ctx context.Context, answer domain.InterviewAnswer) (domain.InterviewAnswer, error) {
	eval, err := json.Marshal(answer.Evaluation)
	if err != nil {
		return domain.InterviewAnswer{}, err
	}
	err = r.db.QueryRow(ctx, `
		insert into interview_answers (session_id, question, answer, topic, difficulty, is_follow_up, question_order, score, evaluation_json)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		returning id, created_at
	`, answer.SessionID, answer.Question, answer.Answer, answer.Topic, answer.Difficulty, answer.IsFollowUp, answer.Order, answer.Score, eval).
		Scan(&answer.ID, &answer.CreatedAt)
	return answer, err
}

func (r *Repository) ListAnswers(ctx context.Context, sessionID int64) ([]domain.InterviewAnswer, error) {
	rows, err := r.db.Query(ctx, `
		select id, session_id, question, answer, topic, difficulty, is_follow_up, question_order, score, evaluation_json, created_at
		from interview_answers where session_id=$1 order by question_order
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.InterviewAnswer
	for rows.Next() {
		ans, err := scanAnswer(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, ans)
	}
	return result, rows.Err()
}

func (r *Repository) ListSessionsByUser(ctx context.Context, userID int64, limit int) ([]domain.InterviewSession, error) {
	rows, err := r.db.Query(ctx, sessionQuery()+` where s.user_id = $1 order by s.started_at desc limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.InterviewSession
	for rows.Next() {
		session, err := r.scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, session)
	}
	return result, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func sessionQuery() string {
	return `select s.id, s.user_id, s.specialization_id, sp.slug, sp.name, s.level, s.mode, s.status,
		s.started_at, s.finished_at, s.overall_score, s.grade, s.interview_language, s.current_question, s.current_topic,
		s.current_difficulty, s.question_count, s.follow_up_count_for_topic, s.plan_json, s.state_json
		from interview_sessions s join specializations sp on sp.id = s.specialization_id`
}

func (r *Repository) scanSession(row scanner) (domain.InterviewSession, error) {
	var s domain.InterviewSession
	var planRaw, stateRaw []byte
	err := row.Scan(&s.ID, &s.UserID, &s.SpecializationID, &s.SpecializationSlug, &s.SpecializationName, &s.Level, &s.Mode, &s.Status,
		&s.StartedAt, &s.FinishedAt, &s.OverallScore, &s.Grade, &s.InterviewLanguage, &s.CurrentQuestion, &s.CurrentTopic,
		&s.CurrentDifficulty, &s.QuestionCount, &s.FollowUpCountForTopic, &planRaw, &stateRaw)
	if err != nil {
		return domain.InterviewSession{}, err
	}
	if err := json.Unmarshal(planRaw, &s.Plan); err != nil {
		return domain.InterviewSession{}, fmt.Errorf("unmarshal plan: %w", err)
	}
	if err := json.Unmarshal(stateRaw, &s.State); err != nil {
		return domain.InterviewSession{}, fmt.Errorf("unmarshal state: %w", err)
	}
	return s, nil
}

func scanAnswer(row scanner) (domain.InterviewAnswer, error) {
	var ans domain.InterviewAnswer
	var evalRaw []byte
	err := row.Scan(&ans.ID, &ans.SessionID, &ans.Question, &ans.Answer, &ans.Topic, &ans.Difficulty, &ans.IsFollowUp, &ans.Order, &ans.Score, &evalRaw, &ans.CreatedAt)
	if err != nil {
		return domain.InterviewAnswer{}, err
	}
	if err := json.Unmarshal(evalRaw, &ans.Evaluation); err != nil {
		return domain.InterviewAnswer{}, err
	}
	return ans, nil
}

func marshalPlanState(session domain.InterviewSession) ([]byte, []byte, error) {
	plan, err := json.Marshal(session.Plan)
	if err != nil {
		return nil, nil, err
	}
	state, err := json.Marshal(session.State)
	if err != nil {
		return nil, nil, err
	}
	return plan, state, nil
}
