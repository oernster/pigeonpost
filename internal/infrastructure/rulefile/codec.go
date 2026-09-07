// Package rulefile is the on-disk format for a set of filter rules: the file a user exports from one
// installation and imports into another. It is JSON because a rules file is a document someone may
// open, read and hand-edit; the shape is a plain tree with no binary in it.
package rulefile

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/oernster/pigeonpost/internal/application"
)

const (
	// kind identifies the file as this application's rules export, so a JSON file of some other shape
	// is refused by name rather than silently read as an empty rule set.
	kind = "pigeonpost.rules"
	// version is the format version written into every file. A reader accepts its own version and
	// below; a file from a later version is refused, since the fields it carries are unknown here.
	version = 1
)

// ErrNotARulesFile is returned when the bytes parse as JSON but are not a rules file.
var ErrNotARulesFile = errors.New("this is not a PigeonPost rules file")

// ErrFutureVersion is returned when the file was written by a newer version of the format.
var ErrFutureVersion = errors.New("this rules file was written by a newer version of PigeonPost")

// document is the file itself: a small header naming what it is, then the rules in evaluation order.
type document struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Rules   []rule `json:"rules"`
}

type rule struct {
	Name           string      `json:"name"`
	Enabled        bool        `json:"enabled"`
	MatchMode      string      `json:"matchMode"`
	StopProcessing bool        `json:"stopProcessing"`
	Accounts       []string    `json:"accounts"`
	Conditions     []condition `json:"conditions"`
	Actions        []action    `json:"actions"`
}

type condition struct {
	Field     string `json:"field"`
	Operator  string `json:"operator"`
	Text      string `json:"text"`
	MatchCase bool   `json:"matchCase"`
	Not       bool   `json:"not"`
}

// action names a move's destination as an account plus a mailbox path. The two together are the folder
// id, so nothing is lost, while the file stays readable and an import can say which folder a rule
// wanted when this installation does not hold it.
type action struct {
	Kind    string `json:"kind"`
	Account string `json:"account,omitempty"`
	Folder  string `json:"folder,omitempty"`
}

// Codec reads and writes the rules file format. It holds no state.
type Codec struct{}

// New constructs the codec.
func New() Codec { return Codec{} }

// Encode writes the rules as an indented JSON document, indented because the file is meant to be
// readable by the person who exported it.
func (Codec) Encode(rules []application.RuleTransfer) ([]byte, error) {
	doc := document{Kind: kind, Version: version, Rules: make([]rule, 0, len(rules))}
	for _, r := range rules {
		doc.Rules = append(doc.Rules, toFileRule(r))
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("rulefile: encode: %w", err)
	}
	return append(data, '\n'), nil
}

// Decode reads a rules file. It refuses anything that is not one, rather than returning an empty set:
// importing "no rules" from a file the user believed held theirs is a silent wrong answer.
func (Codec) Decode(data []byte) ([]application.RuleTransfer, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("rulefile: decode: %w", err)
	}
	if doc.Kind != kind {
		return nil, ErrNotARulesFile
	}
	if doc.Version > version {
		return nil, ErrFutureVersion
	}
	out := make([]application.RuleTransfer, 0, len(doc.Rules))
	for _, r := range doc.Rules {
		out = append(out, fromFileRule(r))
	}
	return out, nil
}

func toFileRule(r application.RuleTransfer) rule {
	conditions := make([]condition, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		conditions = append(conditions, condition{
			Field: c.Field, Operator: c.Operator, Text: c.Text, MatchCase: c.MatchCase, Not: c.Not,
		})
	}
	actions := make([]action, 0, len(r.Actions))
	for _, a := range r.Actions {
		actions = append(actions, action{Kind: a.Kind, Account: a.Account, Folder: a.Folder})
	}
	accounts := r.Accounts
	if accounts == nil {
		accounts = []string{}
	}
	return rule{
		Name: r.Name, Enabled: r.Enabled, MatchMode: r.MatchMode, StopProcessing: r.StopProcessing,
		Accounts: accounts, Conditions: conditions, Actions: actions,
	}
}

func fromFileRule(r rule) application.RuleTransfer {
	conditions := make([]application.RuleTransferCondition, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		conditions = append(conditions, application.RuleTransferCondition{
			Field: c.Field, Operator: c.Operator, Text: c.Text, MatchCase: c.MatchCase, Not: c.Not,
		})
	}
	actions := make([]application.RuleTransferAction, 0, len(r.Actions))
	for _, a := range r.Actions {
		actions = append(actions, application.RuleTransferAction{Kind: a.Kind, Account: a.Account, Folder: a.Folder})
	}
	return application.RuleTransfer{
		Name: r.Name, Enabled: r.Enabled, MatchMode: r.MatchMode, StopProcessing: r.StopProcessing,
		Accounts: r.Accounts, Conditions: conditions, Actions: actions,
	}
}
