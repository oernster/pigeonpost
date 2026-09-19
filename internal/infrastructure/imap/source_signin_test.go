package imap

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// A refusal is marked so the interface can replace the server's own line with a sentence. The marking is
// driven through the real client here rather than asserted against a hand-built error, because what is
// being proved is that the library reports a tagged NO in the shape the marking reads.
func TestVerifyMarksARefusedSignIn(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{loginRefusal: "[AUTHENTICATIONFAILED] Invalid credentials (Failure)"})

	err := fakeSource().Verify(context.Background(), fakeAccount(t, host, port), "wrong")
	if err == nil {
		t.Fatal("a refused sign-in was reported as success")
	}
	if !errors.Is(err, domain.ErrSignInRefused) {
		t.Fatalf("a refused sign-in was not marked for the interface: %v", err)
	}
	if errors.Is(err, domain.ErrAppPasswordRequired) {
		t.Fatalf("a generic refusal was read as the server asking for an app password: %v", err)
	}
}

// Where the server names the remedy itself, the narrower sentinel rides alongside the refusal, so the
// interface can say what was asked for instead of listing what it might be.
func TestVerifyMarksAnAppPasswordRefusal(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{
		loginRefusal: "[ALERT] Application-specific password required: https://support.google.com/accounts/answer/185833",
	})

	err := fakeSource().Verify(context.Background(), fakeAccount(t, host, port), "account-password")
	if err == nil {
		t.Fatal("a refused sign-in was reported as success")
	}
	if !errors.Is(err, domain.ErrAppPasswordRequired) {
		t.Fatalf("the server's own statement was not marked: %v", err)
	}
	if !errors.Is(err, domain.ErrSignInRefused) {
		t.Fatalf("the app-password refusal is still a refused sign-in: %v", err)
	}
}

// A sign-in that works is not marked, so nothing downstream reads a refusal into a healthy account.
func TestVerifyLeavesAWorkingSignInUnmarked(t *testing.T) {
	t.Parallel()
	host, port := listenFake(t, script{})

	if err := fakeSource().Verify(context.Background(), fakeAccount(t, host, port), "secret"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

// markRefusal is narrow on purpose: a failure that is not a tagged refusal keeps its own detail, so a
// genuine fault still surfaces rather than being dressed as a bad password.
func TestMarkRefusalLeavesOtherFailuresAlone(t *testing.T) {
	t.Parallel()
	if markRefusal(nil) != nil {
		t.Fatal("nil was marked")
	}
	ordinary := errors.New("write tcp 127.0.0.1:1: use of closed network connection")
	if errors.Is(markRefusal(ordinary), domain.ErrSignInRefused) {
		t.Fatal("a dropped connection was marked as a refused sign-in")
	}
}
