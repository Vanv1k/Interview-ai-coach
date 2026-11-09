package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"ai-interview-coach/internal/domain"
	"ai-interview-coach/internal/interview"
	"ai-interview-coach/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Bot struct {
	bot     *bot.Bot
	service *service.InterviewService
	logger  *slog.Logger
	mu      sync.Mutex
	drafts  map[int64]draft
}

type draft struct {
	SpecializationSlug string
	Level              domain.Level
	Mode               domain.Mode
}

func NewBot(token string, interviewService *service.InterviewService, logger *slog.Logger) (*Bot, error) {
	app := &Bot{service: interviewService, logger: logger, drafts: map[int64]draft{}}
	b, err := bot.New(token)
	if err != nil {
		return nil, err
	}
	app.bot = b
	app.register()
	return app, nil
}

func (b *Bot) Run(ctx context.Context) error {
	b.bot.Start(ctx)
	return ctx.Err()
}

func (b *Bot) register() {
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, b.handleStart)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/interview", bot.MatchTypeExact, b.handleInterview)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, b.handleHelp)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/stop", bot.MatchTypeExact, b.handleStop)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/history", bot.MatchTypeExact, b.handleHistory)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "start_interview", bot.MatchTypePrefix, b.handleCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "spec:", bot.MatchTypePrefix, b.handleCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "level:", bot.MatchTypePrefix, b.handleCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "mode:", bot.MatchTypePrefix, b.handleCallback)
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "begin", bot.MatchTypeExact, b.handleCallback)
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, b.handleAnswer)
}

func (b *Bot) handleStart(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	text := "AI Interview Coach проводит реалистичную имитацию технического собеседования.\n\nВыберите параметры, отвечайте как на реальном интервью, а в конце получите score, grade и подробный feedback."
	b.send(ctx, update.Message.Chat.ID, text, startKeyboard())
}

func (b *Bot) handleInterview(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	b.send(ctx, update.Message.Chat.ID, "Выберите направление:", directionKeyboard())
}

func (b *Bot) handleHelp(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	text := "/start - описание продукта\n/interview - начать новое интервью\n/stop - остановить активное интервью\n/history - последние интервью\n/help - помощь\n\nВо время активного интервью просто отправляйте ответы текстом."
	b.send(ctx, update.Message.Chat.ID, text, nil)
}

func (b *Bot) handleStop(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if err := b.service.StopActive(ctx, update.Message.From.ID, username(update.Message.From)); err != nil {
		b.send(ctx, update.Message.Chat.ID, "Активное интервью не найдено.", nil)
		return
	}
	b.send(ctx, update.Message.Chat.ID, "Интервью остановлено.", nil)
}

func (b *Bot) handleHistory(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	sessions, err := b.service.History(ctx, update.Message.From.ID, username(update.Message.From), 5)
	if err != nil {
		b.logger.Error("history failed", "error", err)
		b.send(ctx, update.Message.Chat.ID, "Не удалось получить историю.", nil)
		return
	}
	if len(sessions) == 0 {
		b.send(ctx, update.Message.Chat.ID, "История интервью пока пустая.", nil)
		return
	}
	var out strings.Builder
	out.WriteString("Последние интервью:\n")
	for _, s := range sessions {
		score := "-"
		if s.OverallScore != nil {
			score = strconv.Itoa(*s.OverallScore)
		}
		fmt.Fprintf(&out, "- %s %s, %s, status=%s, score=%s, grade=%s\n", s.SpecializationName, s.Level, s.Mode, s.Status, score, emptyDash(s.Grade))
	}
	b.send(ctx, update.Message.Chat.ID, out.String(), nil)
}

func (b *Bot) handleCallback(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	cb := update.CallbackQuery
	if cb == nil {
		return
	}
	_, _ = botAPI.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: cb.ID})
	if cb.Message.Message == nil {
		return
	}
	chatID := cb.Message.Message.Chat.ID
	userID := cb.From.ID
	data := cb.Data

	switch {
	case data == "start_interview":
		b.send(ctx, chatID, "Выберите направление:", directionKeyboard())
	case strings.HasPrefix(data, "spec:"):
		spec := strings.TrimPrefix(data, "spec:")
		b.updateDraft(userID, func(d *draft) { d.SpecializationSlug = spec })
		b.send(ctx, chatID, "Выберите уровень:", levelKeyboard())
	case strings.HasPrefix(data, "level:"):
		level := domain.Level(strings.TrimPrefix(data, "level:"))
		b.updateDraft(userID, func(d *draft) { d.Level = level })
		b.send(ctx, chatID, "Выберите длительность:", modeKeyboard())
	case strings.HasPrefix(data, "mode:"):
		mode := domain.Mode(strings.TrimPrefix(data, "mode:"))
		b.updateDraft(userID, func(d *draft) { d.Mode = mode })
		d := b.getDraft(userID)
		b.send(ctx, chatID, fmt.Sprintf("Начинаем техническое интервью:\n%s — %s\n\nЯ буду задавать вопросы как интервьюер. Отвечай так, как отвечал бы на реальном собеседовании.", titleSpec(d.SpecializationSlug), d.Level), beginKeyboard())
	case data == "begin":
		d := b.getDraft(userID)
		if d.Mode == "" {
			d.Mode = domain.ModeStandard
		}
		res, err := b.service.StartInterview(ctx, service.StartInput{
			TelegramID:         userID,
			Username:           usernameValue(cb.From),
			SpecializationSlug: d.SpecializationSlug,
			Level:              d.Level,
			Mode:               d.Mode,
			LanguageCode:       cb.From.LanguageCode,
		})
		if err != nil {
			if errors.Is(err, service.ErrRateLimited) {
				b.send(ctx, chatID, "Слишком много попыток начать интервью. Подождите немного и попробуйте снова.", nil)
				return
			}
			b.logger.Error("start interview failed", "error", err)
			b.send(ctx, chatID, "Не удалось начать интервью. Проверьте параметры и попробуйте снова.", nil)
			return
		}
		b.send(ctx, chatID, fmt.Sprintf("Вопрос 1:\n%s", res.Question.Question), nil)
	}
}

func (b *Bot) handleAnswer(ctx context.Context, botAPI *bot.Bot, update *models.Update) {
	if update.Message == nil || strings.HasPrefix(strings.TrimSpace(update.Message.Text), "/") {
		return
	}
	res, err := b.service.SubmitAnswer(ctx, update.Message.From.ID, username(update.Message.From), update.Message.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNoActiveSession):
			b.send(ctx, update.Message.Chat.ID, "Активное интервью не найдено. Используйте /interview, чтобы начать.", nil)
		case errors.Is(err, service.ErrRateLimited):
			b.send(ctx, update.Message.Chat.ID, "Слишком много сообщений. Подождите немного и отправьте ответ ещё раз.", nil)
		case errors.Is(err, interview.ErrEmptyAnswer):
			b.send(ctx, update.Message.Chat.ID, "Ответ пустой. Напишите ответ текстом.", nil)
		case errors.Is(err, interview.ErrAnswerTooLong):
			b.send(ctx, update.Message.Chat.ID, "Ответ слишком длинный. Сформулируйте короче.", nil)
		default:
			b.logger.Error("submit answer failed", "error", err)
			b.send(ctx, update.Message.Chat.ID, "Не удалось обработать ответ. Попробуйте ещё раз позже.", nil)
		}
		return
	}
	if res.FinalFeedback != nil {
		b.send(ctx, update.Message.Chat.ID, "Интервью завершено.\n\n"+service.FormatFeedback(*res.FinalFeedback), nil)
		return
	}
	if res.NextQuestion != nil {
		b.send(ctx, update.Message.Chat.ID, fmt.Sprintf("Следующий вопрос:\n%s", res.NextQuestion.Question), nil)
	}
}

func (b *Bot) send(ctx context.Context, chatID int64, text string, markup any) {
	params := &bot.SendMessageParams{ChatID: chatID, Text: text}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := b.bot.SendMessage(ctx, params); err != nil {
		b.logger.Error("telegram send failed", "error", err)
	}
}

func (b *Bot) updateDraft(userID int64, fn func(*draft)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.drafts[userID]
	fn(&d)
	b.drafts[userID] = d
}

func (b *Bot) getDraft(userID int64) draft {
	b.mu.Lock()
	defer b.mu.Unlock()
	d := b.drafts[userID]
	if d.Level == "" {
		d.Level = domain.LevelMiddle
	}
	if d.Mode == "" {
		d.Mode = domain.ModeStandard
	}
	if d.SpecializationSlug == "" {
		d.SpecializationSlug = "go-backend"
	}
	return d
}

func startKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Начать интервью", CallbackData: "start_interview"}}}}
}

func directionKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "Go Backend", CallbackData: "spec:go-backend"}},
		{{Text: "Java Backend", CallbackData: "spec:java-backend"}},
		{{Text: "JS/TS Frontend", CallbackData: "spec:ts-frontend"}},
	}}
}

func levelKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "Junior", CallbackData: "level:junior"}, {Text: "Middle", CallbackData: "level:middle"}, {Text: "Senior", CallbackData: "level:senior"}},
	}}
}

func modeKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{{Text: "Quick", CallbackData: "mode:quick"}, {Text: "Standard", CallbackData: "mode:standard"}, {Text: "Full", CallbackData: "mode:full"}},
	}}
}

func beginKeyboard() models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Начать", CallbackData: "begin"}}}}
}

func username(u *models.User) string {
	if u == nil {
		return ""
	}
	return usernameValue(*u)
}

func usernameValue(u models.User) string {
	if u.Username != "" {
		return u.Username
	}
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

func titleSpec(slug string) string {
	switch slug {
	case "go-backend":
		return "Go Backend"
	case "java-backend":
		return "Java Backend"
	case "ts-frontend":
		return "JavaScript/TypeScript Frontend"
	default:
		return slug
	}
}

func emptyDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
