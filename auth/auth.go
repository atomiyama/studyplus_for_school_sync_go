//go:generate mockgen -source=$GOFILE -destination=mock_$GOFILE -package=$GOPACKAGE
package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/oauth2"
)

// Authorization provides features that help the authorization process and managing tokens,
// such as getting tokens from the data sources and persisting refreshed tokens.
// Most users will use golang.org/x/oauth2 package.
type Authorization struct {
	Config       *oauth2.Config
	tokenManager *tokenManager
}

// TokenStore is anything that can get token and store token, with any datasource. (e.g. cache, database)
type TokenStore interface {
	// Get returns the persisted token from some data source.
	Get() (*oauth2.Token, error)

	// Store persists the token into some data source.
	Save(*oauth2.Token) error
}

func NewAuthorization(cnf *oauth2.Config, ts TokenStore) *Authorization {
	tokenManager := newTokenManager(ts)
	return &Authorization{Config: cnf, tokenManager: tokenManager}
}

func (a *Authorization) AuthCodeURL(state string) string {
	return a.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (a *Authorization) AuthorizeFromCode(ctx context.Context, code string) error {
	token, err := a.Config.Exchange(ctx, code)
	if err != nil {
		return err
	}
	if err := a.tokenManager.Save(token); err != nil {
		return err
	}
	return nil
}

func (a *Authorization) AuthorizeCLI(state string) error {
	url := a.AuthCodeURL(state)
	fmt.Printf("Visit the URL: %s\n", url)
	fmt.Print("Enter authrization code >")
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return err
	}
	ctx := context.Background()
	if err := a.AuthorizeFromCode(ctx, code); err != nil {
		return err
	}
	return nil
}

func (a *Authorization) Client(ctx context.Context) (*http.Client, error) {
	token, err := a.tokenManager.Get()
	if err != nil {
		return nil, err
	}
	src := a.Config.TokenSource(ctx, token)
	a.tokenManager.src = src
	r := oauth2.ReuseTokenSource(token, a.tokenManager)

	ctx = context.Background()
	return oauth2.NewClient(ctx, r), nil
}

type tokenManager struct {
	store TokenStore
	mut   sync.Mutex
	src   oauth2.TokenSource
}

func newTokenManager(store TokenStore) *tokenManager {
	return &tokenManager{store: store}
}

func (s *tokenManager) Token() (*oauth2.Token, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	token, err := s.store.Get()
	if err != nil {
		return nil, err
	}
	if token.Valid() {
		return token, nil
	}
	if err := s.store.Save(token); err != nil {
		return token, err
	}
	return token, nil
}

func (s *tokenManager) Get() (*oauth2.Token, error) {
	return s.store.Get()
}

func (s *tokenManager) Save(t *oauth2.Token) error {
	return s.store.Save(t)
}
