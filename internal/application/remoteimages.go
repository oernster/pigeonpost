package application

import (
	"context"
	"fmt"
)

// RemoteImageService is the use-case boundary for loading a message's blocked remote images. It delegates to a
// resolver that fetches each remote image server-side and inlines it into the HTML as a data: URI, so the
// reader can display images a browser cannot load cross-origin (see the RemoteImageResolver port).
type RemoteImageService struct {
	resolver RemoteImageResolver
}

// NewRemoteImageService constructs the service with its injected resolver.
func NewRemoteImageService(resolver RemoteImageResolver) *RemoteImageService {
	return &RemoteImageService{resolver: resolver}
}

// LoadImages returns the message HTML with its blocked remote images fetched and inlined as data: URIs. The
// resolver leaves any image it cannot fetch parked, so a partial failure still returns usable HTML.
func (s *RemoteImageService) LoadImages(ctx context.Context, html string) (string, error) {
	resolved, err := s.resolver.Resolve(ctx, html)
	if err != nil {
		return "", fmt.Errorf("load remote images: %w", err)
	}
	return resolved, nil
}
