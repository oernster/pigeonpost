package main

import (
	"reflect"
	"testing"
)

func TestFirstMailtoArg(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, ""},
		{"executable only", []string{`C:\Programs\PigeonPost.exe`}, ""},
		{"plain uri", []string{"pp.exe", "mailto:jane@example.org"}, "mailto:jane@example.org"},
		{"uppercase scheme", []string{"pp.exe", "MAILTO:jane@example.org"}, "MAILTO:jane@example.org"},
		{"bare scheme is not a uri", []string{"pp.exe", "mailto:"}, ""},
		{"eml path is ignored", []string{"pp.exe", `C:\mail\note.eml`}, ""},
		{
			"first of two wins",
			[]string{"pp.exe", "mailto:a@example.org", "mailto:b@example.org"},
			"mailto:a@example.org",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := firstMailtoArg(tc.args); got != tc.want {
				t.Fatalf("firstMailtoArg(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestParseMailto(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		uri  string
		want mailtoFields
	}{
		{
			"address only",
			"mailto:jane@example.org",
			mailtoFields{To: []string{"jane@example.org"}},
		},
		{
			"two addresses in the path",
			"mailto:jane@example.org,joe@example.org",
			mailtoFields{To: []string{"jane@example.org", "joe@example.org"}},
		},
		{
			"subject and body decode percent escapes",
			"mailto:jane@example.org?subject=Chess%20move&body=Knight%20takes.%0D%0AYour%20turn.",
			mailtoFields{
				To:      []string{"jane@example.org"},
				Subject: "Chess move",
				Body:    "Knight takes.\r\nYour turn.",
			},
		},
		{
			"cc bcc and extra to accumulate",
			"mailto:jane@example.org?to=joe@example.org&cc=ann@example.org&bcc=raj@example.org",
			mailtoFields{
				To:  []string{"jane@example.org", "joe@example.org"},
				Cc:  []string{"ann@example.org"},
				Bcc: []string{"raj@example.org"},
			},
		},
		{
			"plus in an address is literal, not a space",
			"mailto:?to=user%2Btag@example.org&cc=other+tag@example.org",
			mailtoFields{
				To: []string{"user+tag@example.org"},
				Cc: []string{"other+tag@example.org"},
			},
		},
		{
			"percent-encoded path address",
			"mailto:jane%40example.org",
			mailtoFields{To: []string{"jane@example.org"}},
		},
		{
			"empty query keeps just the address",
			"mailto:jane@example.org?",
			mailtoFields{To: []string{"jane@example.org"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseMailto(tc.uri)
			if err != nil {
				t.Fatalf("parseMailto(%q) error: %v", tc.uri, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseMailto(%q) = %+v, want %+v", tc.uri, got, tc.want)
			}
		})
	}
}

func TestParseMailtoRejectsNonMailto(t *testing.T) {
	t.Parallel()
	for _, uri := range []string{"", "https://example.org", "mail:jane@example.org"} {
		if _, err := parseMailto(uri); err == nil {
			t.Fatalf("parseMailto(%q) succeeded, want error", uri)
		}
	}
}

func TestParseMailtoRejectsMalformedEscapes(t *testing.T) {
	t.Parallel()
	for _, uri := range []string{"mailto:jane%ZZ@example.org", "mailto:j@example.org?subject=%"} {
		if _, err := parseMailto(uri); err == nil {
			t.Fatalf("parseMailto(%q) succeeded, want error", uri)
		}
	}
}
