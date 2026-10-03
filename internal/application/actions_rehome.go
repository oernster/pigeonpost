package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/domain"
)

// rehomed answers msg as it now stands in folderID under uid, keeping all of its data. It fails only
// when the server reported a uid that cannot address a message.
func rehomed(msg domain.MessageSummary, folderID, uid string) (domain.MessageSummary, error) {
	moved, err := domain.NewMessageSummary(domain.MessageSummaryInput{
		ID: domain.MessageIDFor(folderID, uid), FolderID: folderID, UID: uid,
		MessageID: msg.MessageID(), From: msg.From(), To: msg.To(), Cc: msg.Cc(), Subject: msg.Subject(),
		Date: msg.Date(), Size: msg.Size(), Flags: msg.Flags(), HasAttachments: msg.HasAttachments(),
		Snippet: msg.Snippet(), Keywords: msg.Keywords(),
	})
	if err != nil {
		return domain.MessageSummary{}, fmt.Errorf("file moved message %q under uid %q: %w", msg.ID(), uid, err)
	}
	return moved, nil
}

// rehome files moved messages in their destination folders' cache in one step (see
// MailStore.RehomeMessages for why).
func (s *MessageActionService) rehome(ctx context.Context, moved []domain.MessageSummary) error {
	if len(moved) == 0 {
		return nil
	}
	if err := s.store.RehomeMessages(ctx, moved); err != nil {
		return fmt.Errorf("file %d moved message(s) in their destination folder: %w", len(moved), err)
	}
	return nil
}

// landings collects where a bulk move's messages landed: each moved id's new id (so the caller can undo)
// and the message as it now stands there, filed in one step by fileLandings.
type landings struct {
	newIDs  map[string]string
	arrived []domain.MessageSummary
	errs    []error
}

func newLandings() *landings {
	return &landings{newIDs: map[string]string{}}
}

// add records that the message id landed in folderID under uid; a uid that cannot address it is
// recorded as an error instead.
func (l *landings) add(id string, msg domain.MessageSummary, folderID, uid string) {
	landedAs, err := rehomed(msg, folderID, uid)
	if err != nil {
		l.errs = append(l.errs, err)
		return
	}
	l.newIDs[id] = landedAs.ID()
	l.arrived = append(l.arrived, landedAs)
}

// fileLandings files every recorded landing in its destination and reports any landing that failed.
func (s *MessageActionService) fileLandings(ctx context.Context, l *landings) error {
	return errors.Join(append(l.errs, s.rehome(ctx, l.arrived))...)
}

// settleMove finishes a single-message move the server has made: the source row leaves the cache. When
// the server reported where the message landed (COPYUID), the message is filed there under the id it
// now carries, which is returned so the caller can undo the move. A server that reports nothing
// leaves the destination to its next sync and returns an empty id.
func (s *MessageActionService) settleMove(ctx context.Context, msg domain.MessageSummary, destFolderID, newUID string) (string, error) {
	if err := s.store.DeleteMessage(ctx, msg.ID()); err != nil {
		return "", fmt.Errorf("remove moved message %q from cache: %w", msg.ID(), err)
	}
	if newUID == "" {
		return "", nil
	}
	moved, err := rehomed(msg, destFolderID, newUID)
	if err != nil {
		return "", err
	}
	if err := s.rehome(ctx, []domain.MessageSummary{moved}); err != nil {
		return "", err
	}
	return moved.ID(), nil
}
