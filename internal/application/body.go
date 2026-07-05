package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// MessageBodyService is the use-case boundary for reading a message's full body. It serves the cached
// body when present and otherwise fetches it from the server, caches it and returns it, so a message
// reads offline after the first open.
type MessageBodyService struct {
	messages MailStore
	accounts AccountStore
	source   MailSource
}

// NewMessageBodyService constructs the service with its injected store, account store and mail source.
func NewMessageBodyService(messages MailStore, accounts AccountStore, source MailSource) *MessageBodyService {
	return &MessageBodyService{messages: messages, accounts: accounts, source: source}
}

// Body returns a message's full body, fetching and caching it on a cache miss.
func (s *MessageBodyService) Body(ctx context.Context, messageID string) (domain.MessageBody, error) {
	cached, err := s.messages.GetMessageBody(ctx, messageID)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, ErrBodyNotCached) {
		return domain.MessageBody{}, fmt.Errorf("body cache lookup %q: %w", messageID, err)
	}

	msg, err := s.messages.GetMessage(ctx, messageID)
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.messages.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	plain, html, invite, err := s.source.FetchBody(ctx, account, folder, msg.UID())
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("fetch body %q: %w", messageID, err)
	}
	body, err := domain.NewMessageBody(messageID, plain, html)
	if err != nil {
		return domain.MessageBody{}, fmt.Errorf("build body %q: %w", messageID, err)
	}
	body = body.WithInvite(invite)
	if err := s.messages.SaveMessageBody(ctx, body); err != nil {
		return domain.MessageBody{}, fmt.Errorf("cache body %q: %w", messageID, err)
	}
	return body, nil
}

// RawMessage bundles a message's raw RFC822 bytes with its subject, so a caller can attach it to
// another message under a sensible filename.
type RawMessage struct {
	Raw     []byte
	Subject string
}

// Raw returns the full raw RFC822 bytes of a message, fetched from the server. Unlike Body it is not
// cached: it serves one-off export (.eml) and attach-an-email, not repeated reads.
func (s *MessageBodyService) Raw(ctx context.Context, messageID string) ([]byte, error) {
	raw, err := s.rawMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	return raw.Raw, nil
}

// RawMessage returns a message's raw bytes together with its subject, for attaching it to a new
// message as a message/rfc822 part.
func (s *MessageBodyService) RawMessage(ctx context.Context, messageID string) (RawMessage, error) {
	return s.rawMessage(ctx, messageID)
}

// rawMessage locates a message, fetches its raw bytes and returns them with its subject. It is the
// shared core of Raw and RawMessage.
func (s *MessageBodyService) rawMessage(ctx context.Context, messageID string) (RawMessage, error) {
	msg, err := s.messages.GetMessage(ctx, messageID)
	if err != nil {
		return RawMessage{}, fmt.Errorf("locate message %q: %w", messageID, err)
	}
	folder, err := s.messages.GetFolder(ctx, msg.FolderID())
	if err != nil {
		return RawMessage{}, fmt.Errorf("locate folder %q: %w", msg.FolderID(), err)
	}
	account, err := s.accounts.GetAccount(ctx, folder.AccountID())
	if err != nil {
		return RawMessage{}, fmt.Errorf("locate account %q: %w", folder.AccountID(), err)
	}
	raw, err := s.source.FetchRaw(ctx, account, folder, msg.UID())
	if err != nil {
		return RawMessage{}, fmt.Errorf("fetch raw %q: %w", messageID, err)
	}
	return RawMessage{Raw: raw, Subject: msg.Subject()}, nil
}
