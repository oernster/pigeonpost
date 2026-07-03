package application

import (
	"context"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// AccountService is the use-case boundary for reading and adding accounts.
type AccountService struct {
	accounts AccountStore
}

// NewAccountService constructs the service with its injected store.
func NewAccountService(accounts AccountStore) *AccountService {
	return &AccountService{accounts: accounts}
}

// List returns all configured accounts.
func (s *AccountService) List(ctx context.Context) ([]domain.Account, error) {
	accounts, err := s.accounts.ListAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, nil
}

// Add persists a new or updated account.
func (s *AccountService) Add(ctx context.Context, account domain.Account) error {
	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("add account %q: %w", account.ID(), err)
	}
	return nil
}
