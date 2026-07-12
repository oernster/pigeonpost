package application

import (
	"context"
	"errors"
	"testing"
)

func TestRemoteImageServiceLoadsImages(t *testing.T) {
	const resolved = `<img src="data:image/png;base64,AA==">`
	svc := NewRemoteImageService(&fakeRemoteImageResolver{resolved: resolved})
	out, err := svc.LoadImages(context.Background(), `<img data-pp-src="https://x.test/a.png">`)
	if err != nil {
		t.Fatalf("LoadImages: %v", err)
	}
	if out != resolved {
		t.Errorf("expected the resolver's HTML returned, got: %s", out)
	}
}

func TestRemoteImageServiceWrapsResolverError(t *testing.T) {
	sentinel := errors.New("resolver boom")
	svc := NewRemoteImageService(&fakeRemoteImageResolver{err: sentinel})
	_, err := svc.LoadImages(context.Background(), `<img data-pp-src="https://x.test/a.png">`)
	if err == nil {
		t.Fatal("expected an error when the resolver fails")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected the resolver error wrapped, got: %v", err)
	}
}
