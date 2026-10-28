package cron

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"mega-trainer-go/internal/domain"
)

type fakeUserRepo struct {
	users []domain.User
	err   error
}

func (r *fakeUserRepo) Upsert(context.Context, *domain.User) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *fakeUserRepo) GetByTelegramID(context.Context, int64) (*domain.User, error) {
	return nil, domain.ErrNotFound
}

func (r *fakeUserRepo) ListActive(_ context.Context, limit int) ([]*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	users := make([]*domain.User, 0, len(r.users))
	for i := range r.users {
		if limit > 0 && len(users) == limit {
			break
		}
		users = append(users, &r.users[i])
	}
	return users, nil
}

type fakeSender struct {
	mu       sync.Mutex
	chatIDs  []int64
	failFor  map[int64]bool
	notify   chan struct{}
	notifyMu sync.Once
}

func newFakeSender() *fakeSender {
	return &fakeSender{failFor: make(map[int64]bool), notify: make(chan struct{})}
}

func (s *fakeSender) SendMessage(chatID int64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.failFor[chatID] {
		return errors.New("telegram is unavailable")
	}

	s.chatIDs = append(s.chatIDs, chatID)
	s.notifyMu.Do(func() { close(s.notify) })
	return nil
}

func (s *fakeSender) sent() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int64(nil), s.chatIDs...)
}

func TestRunnerOnceSendsToEveryActiveUser(t *testing.T) {
	t.Parallel()

	repo := &fakeUserRepo{users: []domain.User{
		{ID: 1, TelegramID: 1001},
		{ID: 2, TelegramID: 1002},
	}}
	sender := newFakeSender()

	runner := New(repo, sender, zerolog.Nop(), time.Hour)
	if err := runner.Once(context.Background()); err != nil {
		t.Fatalf("once: %v", err)
	}

	sent := sender.sent()
	if len(sent) != 2 || sent[0] != 1001 || sent[1] != 1002 {
		t.Fatalf("sent to %v, want [1001 1002]", sent)
	}
}

func TestRunnerOnceIsolatesDeliveryFailures(t *testing.T) {
	t.Parallel()

	repo := &fakeUserRepo{users: []domain.User{
		{ID: 1, TelegramID: 1001},
		{ID: 2, TelegramID: 1002},
	}}
	sender := newFakeSender()
	sender.failFor[1001] = true

	runner := New(repo, sender, zerolog.Nop(), time.Hour)
	if err := runner.Once(context.Background()); err != nil {
		t.Fatalf("once: %v", err)
	}

	sent := sender.sent()
	if len(sent) != 1 || sent[0] != 1002 {
		t.Fatalf("sent to %v, want [1002]", sent)
	}
}

func TestRunnerOnceReturnsRepositoryError(t *testing.T) {
	t.Parallel()

	repo := &fakeUserRepo{err: errors.New("database is down")}
	runner := New(repo, newFakeSender(), zerolog.Nop(), time.Hour)

	if err := runner.Once(context.Background()); err == nil {
		t.Fatal("expected an error when the repository fails")
	}
}

func TestRunnerStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	repo := &fakeUserRepo{users: []domain.User{{ID: 1, TelegramID: 1001}}}
	sender := newFakeSender()

	// A very short interval makes the first tick arrive quickly.
	runner := New(repo, sender, zerolog.Nop(), time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()

	select {
	case <-sender.notify:
	case <-time.After(2 * time.Second):
		t.Fatal("reminder was not sent within the timeout")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop after the context was cancelled")
	}
}
