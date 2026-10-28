package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"mega-trainer-go/internal/domain"
)

// memUserRepo is an in-memory domain.UserRepository for tests.
type memUserRepo struct {
	mu    sync.Mutex
	users map[int64]*domain.User
	next  int64
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{users: make(map[int64]*domain.User)}
}

func (r *memUserRepo) Upsert(_ context.Context, user *domain.User) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored := *user
	if existing, ok := r.users[user.TelegramID]; ok {
		stored.ID = existing.ID
	} else {
		r.next++
		stored.ID = r.next
	}
	r.users[user.TelegramID] = &stored

	copied := stored
	return &copied, nil
}

func (r *memUserRepo) ListActive(_ context.Context, limit int) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	users := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		if limit > 0 && len(users) == limit {
			break
		}
		copied := *user
		users = append(users, &copied)
	}
	return users, nil
}

func (r *memUserRepo) GetByTelegramID(_ context.Context, telegramID int64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[telegramID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *user
	return &copied, nil
}

func TestUserServiceEnsure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMemUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Ensure(ctx, 1001, "gopher", "Gordon", "Freeman", "ru")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if user.TelegramID != 1001 || user.Username != "gopher" {
		t.Fatalf("user = %+v, want telegram id 1001 and username gopher", user)
	}

	// A second call updates the same user instead of creating a duplicate.
	updated, err := svc.Ensure(ctx, 1001, "gopher2", "Gordon", "Freeman", "ru")
	if err != nil {
		t.Fatalf("ensure second time: %v", err)
	}
	if updated.ID != user.ID {
		t.Fatalf("id = %d, want %d", updated.ID, user.ID)
	}
	if updated.Username != "gopher2" {
		t.Fatalf("username = %q, want gopher2", updated.Username)
	}

	if _, err := repo.GetByTelegramID(ctx, 4242); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestUserServiceRequiresTelegramID(t *testing.T) {
	t.Parallel()

	svc := NewUserService(newMemUserRepo())

	if _, err := svc.Ensure(context.Background(), 0, "", "", "", ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("error = %v, want ErrValidation", err)
	}
}
