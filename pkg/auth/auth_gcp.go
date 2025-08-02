package auth

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

type (
	GCPAuthenticator interface {
		Authenticate(ctx context.Context) (option.ClientOption, error)
	}

	gcpAuth struct {
		fileKey string
		scopes  []string
	}
)

var DefaultScopes = []string{
	"https://www.googleapis.com/auth/cloud-billing",
	"https://www.googleapis.com/auth/cloud-billing.readonly",
	"https://www.googleapis.com/auth/cloud-platform",
}

func NewGCPAuthenticator(keyFile string, scopes ...string) GCPAuthenticator {
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}
	return &gcpAuth{
		fileKey: keyFile,
		scopes:  scopes,
	}
}

func (a *gcpAuth) Authenticate(ctx context.Context) (option.ClientOption, error) {
	data, err := os.ReadFile(a.fileKey)
	if err != nil {
		return nil, fmt.Errorf("read service account key %q: %w", a.fileKey, err)
	}
	creds, err := google.CredentialsFromJSON(ctx, data, a.scopes...)
	if err != nil {
		return nil, fmt.Errorf("parse credentials from JSON: %w", err)
	}
	return option.WithCredentials(creds), nil
}
