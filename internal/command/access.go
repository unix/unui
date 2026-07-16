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
) (store.Credentials, error) {
	var credentials store.Credentials
	err := withCredentialLock(ctx, func() error {
		loaded, loadErr := a.credentialsForRegistry()
		if loadErr != nil {
			return loadErr
		}
		if loaded.AccessToken != rejectedAccessToken && accessTokenReady(loaded) {
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
		if saveErr := store.Save(loaded); saveErr != nil {
			return credentialStoreError(saveErr)
		}
		credentials = loaded
		return nil
	})
	return credentials, err
}

func accessRequest[T any](
	a *app,
	ctx context.Context,
	credentials store.Credentials,
	request func(string) (T, error),
) (T, store.Credentials, error) {
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
