package command

import (
	"context"
	"errors"
	"net/http"

	"github.com/unix/unui/internal/api"
	"github.com/unix/unui/internal/message"
	"github.com/unix/unui/internal/store"
)

func withCredentialLock(
	ctx context.Context,
	action func() error,
) (err error) {
	lock, err := store.Lock(ctx)
	if err != nil {
		return credentialStoreError(err)
	}
	defer func() {
		if unlockErr := lock.Unlock(); err == nil && unlockErr != nil {
			err = credentialStoreError(unlockErr)
		}
	}()
	return action()
}

func (a *app) credentialsAfterUnauthorized(
	ctx context.Context,
	rejectedAccessToken string,
) (scopedCredentials, error) {
	var credentials scopedCredentials
	err := withCredentialLock(ctx, func() error {
		loaded, loadErr := a.credentialsForRegistry()
		if loadErr != nil {
			return loadErr
		}
		if loaded.AccessToken != rejectedAccessToken && accessTokenReady(loaded.RegistryCredentials) {
			credentials = loaded
			return nil
		}
		if loaded.PersonalToken == "" {
			return newCommandError(
				"NOT_LOGGED_IN",
				message.NotLoggedIn(),
				nil,
			)
		}
		if refreshErr := a.refreshAccess(ctx, &loaded); refreshErr != nil {
			return refreshErr
		}
		if setErr := loaded.Credentials.SetRegistry(a.registry, loaded.RegistryCredentials); setErr != nil {
			return credentialStoreError(setErr)
		}
		if saveErr := store.Save(loaded.Credentials); saveErr != nil {
			return credentialStoreError(saveErr)
		}
		credentials = loaded
		return nil
	})
	return credentials, err
}

func updateCredentials(
	ctx context.Context,
	update func(*store.Credentials) error,
) error {
	return withCredentialLock(ctx, func() error {
		credentials, err := store.Load()
		if errors.Is(err, store.ErrNotLoggedIn) {
			credentials = store.Credentials{}
		} else if err != nil {
			return credentialStoreError(err)
		}
		if err := update(&credentials); err != nil {
			return credentialStoreError(err)
		}
		if err := store.Save(credentials); err != nil {
			return credentialStoreError(err)
		}
		return nil
	})
}

func accessRequest[T any](
	a *app,
	ctx context.Context,
	credentials scopedCredentials,
	request func(string) (T, error),
) (T, scopedCredentials, error) {
	result, err := request(credentials.AccessToken)
	if err == nil {
		return result, credentials, nil
	}
	if !isUnauthorizedAPIError(err) {
		var zero T
		return zero, credentials, apiCommandError(err)
	}
	refreshed, refreshErr := a.credentialsAfterUnauthorized(
		ctx,
		credentials.AccessToken,
	)
	if refreshErr != nil {
		var zero T
		return zero, credentials, refreshErr
	}
	result, err = request(refreshed.AccessToken)
	if err != nil {
		var zero T
		return zero, refreshed, apiCommandError(err)
	}
	return result, refreshed, nil
}

func isUnauthorizedAPIError(err error) bool {
	var apiErr *api.Error
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized
}
