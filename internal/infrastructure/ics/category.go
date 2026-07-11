package ics

import (
	"strings"

	goical "github.com/emersion/go-ical"
)

// primaryCategory reads the event's primary category from the CATEGORIES property: the first non-empty
// comma-separated value, trimmed and lowercased, or an empty string when the property is absent or holds
// only blanks. A multi-value CATEGORIES keeps its extra values in the preserved Extra for a lossless
// round-trip; only the primary one is modelled.
func primaryCategory(props goical.Props) string {
	c := props.Get(goical.PropCategories)
	if c == nil {
		return ""
	}
	for _, v := range strings.Split(c.Value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			return strings.ToLower(v)
		}
	}
	return ""
}

// setCategory writes the primary category into the CATEGORIES property while keeping any extra values a
// preserved component already carried, so a multi-value CATEGORIES survives an in-app edit that only sets
// the primary one. The primary value takes the first slot; the rest follow with any duplicate of the new
// primary dropped. An empty category removes the property (dropping the preserved rest with it, so
// clearing the field in the app clears it in the export).
func setCategory(comp *goical.Component, category string) {
	var rest []string
	if existing := comp.Props.Get(goical.PropCategories); existing != nil {
		var parts []string
		for _, v := range strings.Split(existing.Value, ",") {
			if v = strings.TrimSpace(v); v != "" {
				parts = append(parts, v)
			}
		}
		if len(parts) > 1 {
			rest = parts[1:]
		}
	}
	values := make([]string, 0, len(rest)+1)
	if category != "" {
		values = append(values, category)
	}
	for _, v := range rest {
		if category != "" && strings.EqualFold(v, category) {
			continue
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		comp.Props.Del(goical.PropCategories)
		return
	}
	p := goical.NewProp(goical.PropCategories)
	p.Value = strings.Join(values, ",")
	comp.Props.Set(p)
}
