package oauth

import "testing"

func TestXOAUTH2Start(t *testing.T) {
	client := NewXOAUTH2Client("user@example.com", "tok123")
	mech, ir, err := client.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if mech != "XOAUTH2" {
		t.Errorf("mech = %q, want XOAUTH2", mech)
	}
	want := "user=user@example.com\x01auth=Bearer tok123\x01\x01"
	if string(ir) != want {
		t.Errorf("initial response = %q, want %q", ir, want)
	}
}

func TestXOAUTH2Next(t *testing.T) {
	client := NewXOAUTH2Client("user@example.com", "tok")
	resp, err := client.Next([]byte("some error challenge"))
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	// The reply must be empty but non-nil, so an empty line is sent for the server to fail the exchange.
	if resp == nil || len(resp) != 0 {
		t.Errorf("Next response = %v, want a non-nil empty slice", resp)
	}
}
