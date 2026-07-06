package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oernster/pigeonpost/internal/domain"
)

// errPasswordRequired is returned when the wizard submits an empty password.
var errPasswordRequired = errors.New("password is required")

// AccountSetupRequest is the front-end payload from the account-setup wizard. Server settings arrive
// as wire strings that map onto the domain's security and protocol enums.
type AccountSetupRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Protocol    string `json:"protocol"`
	InHost      string `json:"inHost"`
	InPort      int    `json:"inPort"`
	InSecurity  string `json:"inSecurity"`
	OutHost     string `json:"outHost"`
	OutPort     int    `json:"outPort"`
	OutSecurity string `json:"outSecurity"`
}

// AddAccount validates the wizard payload, builds a domain account and configures it: the incoming
// server is verified, the password is stored in the keychain and only then is the account persisted.
// The account id is its email address, so re-adding the same address updates its settings in place.
func (a *App) AddAccount(req AccountSetupRequest) error {
	secret := strings.TrimSpace(req.Password)
	if secret == "" {
		return errPasswordRequired
	}
	account, err := buildAccount(req)
	if err != nil {
		return err
	}
	if err := a.setup.Configure(a.ctx, account, secret); err != nil {
		return err
	}
	// Start the IDLE watcher now so an account added after launch gets instant push straight away rather
	// than waiting for the next restart; a POP3 account is a no-op and stays on the backstop poll.
	a.startMailWatcher(account)
	return nil
}

// UpdateAccount re-configures an existing account from the edit wizard. A blank password keeps the
// current one; the identity (email address) is fixed, so the front end locks it in edit mode.
func (a *App) UpdateAccount(req AccountSetupRequest) error {
	account, err := buildAccount(req)
	if err != nil {
		return err
	}
	if err := a.setup.Update(a.ctx, account, strings.TrimSpace(req.Password)); err != nil {
		return err
	}
	// Restart the watcher so changed server settings take effect without a restart, and a switch to POP3
	// leaves no stale IMAP watcher running.
	a.startMailWatcher(account)
	return nil
}

// parseProtocol maps a wire protocol identifier to the domain Protocol, defaulting to IMAP when the
// value is empty or unrecognised so a payload without the field still builds an IMAP account.
func parseProtocol(value string) domain.Protocol {
	if strings.EqualFold(strings.TrimSpace(value), domain.ProtocolPOP3.String()) {
		return domain.ProtocolPOP3
	}
	return domain.ProtocolIMAP
}

// buildAccount maps a wizard payload to a validated domain account (IMAP or POP3, password auth for v1).
func buildAccount(req AccountSetupRequest) (domain.Account, error) {
	address, err := domain.NewEmailAddress("", strings.TrimSpace(req.Email))
	if err != nil {
		return domain.Account{}, fmt.Errorf("invalid email address: %w", err)
	}
	inSecurity, err := parseSecurity(req.InSecurity)
	if err != nil {
		return domain.Account{}, err
	}
	outSecurity, err := parseSecurity(req.OutSecurity)
	if err != nil {
		return domain.Account{}, err
	}
	incoming, err := domain.NewServerConfig(req.InHost, req.InPort, inSecurity)
	if err != nil {
		return domain.Account{}, fmt.Errorf("incoming server: %w", err)
	}
	outgoing, err := domain.NewServerConfig(req.OutHost, req.OutPort, outSecurity)
	if err != nil {
		return domain.Account{}, fmt.Errorf("outgoing server: %w", err)
	}
	account, err := domain.NewAccount(
		address.Address(), req.DisplayName, address,
		parseProtocol(req.Protocol), incoming, outgoing, domain.AuthPassword,
	)
	if err != nil {
		return domain.Account{}, fmt.Errorf("build account: %w", err)
	}
	return account, nil
}

// parseSecurity maps a wire security identifier to the domain Security enum.
func parseSecurity(value string) (domain.Security, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case domain.SecurityTLS.String():
		return domain.SecurityTLS, nil
	case domain.SecurityStartTLS.String():
		return domain.SecurityStartTLS, nil
	case domain.SecurityNone.String():
		return domain.SecurityNone, nil
	default:
		return domain.SecurityTLS, fmt.Errorf("unknown security mode %q", value)
	}
}
