package application

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// pushKeywordCall is one recorded PushKeyword: the folder, the UIDs, the keyword and whether it was set.
type pushKeywordCall struct {
	folderID string
	uids     []string
	keyword  string
	set      bool
}

// PushKeyword records the batch and settles every UID pushed except those in pushKeywordUnsettled;
// pushKeywordErr fails every call.
func (f *fakeMailActions) PushKeyword(_ context.Context, _ domain.Account, folder domain.Folder, uids []string, keyword string, set bool) ([]string, error) {
	if f.pushKeywordErr != nil {
		return nil, f.pushKeywordErr
	}
	f.pushKeywordBatches = append(f.pushKeywordBatches, pushKeywordCall{folderID: folder.ID(), uids: uids, keyword: keyword, set: set})
	settled := make([]string, 0, len(uids))
	for _, uid := range uids {
		if !f.pushKeywordUnsettled[uid] {
			settled = append(settled, uid)
		}
	}
	return settled, nil
}
