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
	// Signature is the account's compose signature as HTML, inserted into a new message. It may be empty.
	Signature string `json:"signature"`
	// Identities are the account's alternate sender addresses (aliases it may send as, such as a domain
	// alias that shares this mailbox). Each is an address with an optional display name; the list may be
	// empty.
	Identities []IdentityInput `json:"identities"`
}

// IdentityInput is one alternate sender address from the setup wizard: an email address with an optional
// display name.
type IdentityInput struct {
	Name    string `json:"name"`
	Address string `json:"address"`
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

// SignInMicrosoft runs the interactive Microsoft OAuth sign-in: it opens the system browser for consent,
// receives the redirect on a loopback listener, verifies mailbox access with the returned token, stores
// the token in the keychain and persists the account. displayName may be blank, in which case the
// signed-in address is used. It starts the account's IDLE watcher on success so new mail pushes straight
// away; it also returns the signed-in address so the front end can select the new account.
func (a *App) SignInMicrosoft(displayName string) (string, error) {
	account, err := a.msSetup.Configure(a.ctx, strings.TrimSpace(displayName))
	if err != nil {
		return "", err
	}
	a.startMailWatcher(account)
	return account.Address().Address(), nil
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

// Microsoft's fixed IMAP and SMTP endpoints for personal and work/school accounts. They are constant for
// every Microsoft account, so an OAuth sign-in needs no server entry from the user.
const (
	microsoftIMAPHost = "outlook.office365.com"
	microsoftIMAPPort = 993
	microsoftSMTPHost = "smtp.office365.com"
	microsoftSMTPPort = 587
)

// buildMicrosoftAccount builds a validated OAuth IMAP account against Microsoft's fixed servers, for a
// signed-in address. The display name falls back to the address when none is given. It is the
// application.MicrosoftAccountBuilder wired into the setup service, kept here alongside buildAccount so
// the provider server details live in one place.
func buildMicrosoftAccount(email, displayName string) (domain.Account, error) {
	address, err := domain.NewEmailAddress("", strings.TrimSpace(email))
	if err != nil {
		return domain.Account{}, fmt.Errorf("invalid microsoft address: %w", err)
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = address.Address()
	}
	incoming, err := domain.NewServerConfig(microsoftIMAPHost, microsoftIMAPPort, domain.SecurityTLS)
	if err != nil {
		return domain.Account{}, fmt.Errorf("microsoft incoming server: %w", err)
	}
	outgoing, err := domain.NewServerConfig(microsoftSMTPHost, microsoftSMTPPort, domain.SecurityStartTLS)
	if err != nil {
		return domain.Account{}, fmt.Errorf("microsoft outgoing server: %w", err)
	}
	account, err := domain.NewAccount(
		address.Address(), name, address,
		domain.ProtocolIMAP, incoming, outgoing, domain.AuthOAuth2,
	)
	if err != nil {
		return domain.Account{}, fmt.Errorf("build microsoft account: %w", err)
	}
	return account, nil
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
	identities, err := parseIdentities(req.Identities)
	if err != nil {
		return domain.Account{}, err
	}
	return account.WithSignature(req.Signature).WithIdentities(identities), nil
}

// parseIdentities maps the wizard's identity inputs to validated addresses, skipping blank rows and
// rejecting a malformed one so the user is told rather than silently dropping it.
func parseIdentities(inputs []IdentityInput) ([]domain.EmailAddress, error) {
	out := make([]domain.EmailAddress, 0, len(inputs))
	for _, in := range inputs {
		address := strings.TrimSpace(in.Address)
		if address == "" {
			continue
		}
		addr, err := domain.NewEmailAddress(strings.TrimSpace(in.Name), address)
		if err != nil {
			return nil, fmt.Errorf("invalid identity address %q: %w", address, err)
		}
		out = append(out, addr)
	}
	return out, nil
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
