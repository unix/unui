package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/unix/unui-cli/internal/proof"
)

type Error struct {
	Status  int
	Message string
	Body    any
}

func (e *Error) Error() string {
	return fmt.Sprintf("API request failed (%d): %s", e.Status, e.Message)
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

type CreateAuthorizationRequest struct {
	DeviceID          string `json:"deviceId"`
	DeviceName        string `json:"deviceName"`
	Platform          string `json:"platform"`
	PublicKey         string `json:"publicKey"`
	VerifierChallenge string `json:"verifierChallenge"`
}

type CreateAuthorizationResponse struct {
	AuthorizeURL    string    `json:"authorizeUrl"`
	ExpiresAt       time.Time `json:"expiresAt"`
	IntervalSeconds int       `json:"intervalSeconds"`
	PollSecret      string    `json:"pollSecret"`
	RequestID       string    `json:"requestId"`
}

type PollAuthorizationResponse struct {
	ExpiresAt time.Time `json:"expiresAt"`
	Status    string    `json:"status"`
}

type ExchangeAuthorizationRequest struct {
	PollSecret string      `json:"pollSecret"`
	Proof      proof.Value `json:"proof"`
	Verifier   string      `json:"verifier"`
}

type ExchangeAuthorizationResponse struct {
	Device        any       `json:"device"`
	ExpiresAt     time.Time `json:"expiresAt"`
	PersonalToken string    `json:"personalToken"`
}

type AccessTokenRequest struct {
	DeviceID string      `json:"deviceId"`
	Proof    proof.Value `json:"proof"`
}

type AccessTokenResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
	TokenType            string    `json:"tokenType"`
}

type AuthShowResponse struct {
	Account struct {
		Email string `json:"email"`
		ID    string `json:"id"`
	} `json:"account"`
	Device struct {
		DeviceID   string `json:"deviceId"`
		DeviceName string `json:"deviceName"`
		Platform   string `json:"platform"`
	} `json:"device"`
	Membership struct {
		AccessLevel      string     `json:"accessLevel"`
		CanUseCLI        bool       `json:"canUseCli"`
		ExpiresAt        *time.Time `json:"expiresAt"`
		IsAdmin          bool       `json:"isAdmin"`
		IsMember         bool       `json:"isMember"`
		Level            string     `json:"level"`
		MembershipLevel  string     `json:"membershipLevel"`
		RemainingSeconds int64      `json:"remainingSeconds"`
		RequiredCLILevel string     `json:"requiredCliLevel"`
	} `json:"membership"`
	PersonalTokenExpiresAt time.Time `json:"personalTokenExpiresAt"`
}

func (c Client) CreateAuthorization(
	ctx context.Context,
	input CreateAuthorizationRequest,
) (CreateAuthorizationResponse, error) {
	var output CreateAuthorizationResponse
	err := c.do(ctx, http.MethodPost, "/auth/requests", nil, input, &output)
	return output, err
}

func (c Client) PollAuthorization(
	ctx context.Context,
	requestID string,
	pollSecret string,
) (PollAuthorizationResponse, error) {
	var output PollAuthorizationResponse
	err := c.do(
		ctx,
		http.MethodGet,
		"/auth/requests/"+requestID,
		map[string]string{"X-unUI-Poll-Secret": pollSecret},
		nil,
		&output,
	)
	return output, err
}

func (c Client) ExchangeAuthorization(
	ctx context.Context,
	requestID string,
	input ExchangeAuthorizationRequest,
) (ExchangeAuthorizationResponse, error) {
	var output ExchangeAuthorizationResponse
	err := c.do(
		ctx,
		http.MethodPost,
		"/auth/requests/"+requestID+"/exchange",
		nil,
		input,
		&output,
	)
	return output, err
}

func (c Client) IssueAccessToken(
	ctx context.Context,
	personalToken string,
	input AccessTokenRequest,
) (AccessTokenResponse, error) {
	var output AccessTokenResponse
	err := c.do(
		ctx,
		http.MethodPost,
		"/auth/access-tokens",
		map[string]string{"Authorization": "Personal " + personalToken},
		input,
		&output,
	)
	return output, err
}

func (c Client) Logout(
	ctx context.Context,
	personalToken string,
	input AccessTokenRequest,
) error {
	return c.do(
		ctx,
		http.MethodDelete,
		"/auth/personal-token",
		map[string]string{"Authorization": "Personal " + personalToken},
		input,
		nil,
	)
}

func (c Client) ShowAuth(
	ctx context.Context,
	accessToken string,
) (AuthShowResponse, error) {
	var output AuthShowResponse
	err := c.do(
		ctx,
		http.MethodGet,
		"/status",
		map[string]string{"Authorization": "Bearer " + accessToken},
		nil,
		&output,
	)
	return output, err
}

func (c Client) Ask(
	ctx context.Context,
	accessToken string,
	input map[string]any,
) (json.RawMessage, error) {
	var output json.RawMessage
	err := c.do(
		ctx,
		http.MethodPost,
		"/ask",
		map[string]string{"Authorization": "Bearer " + accessToken},
		input,
		&output,
	)
	return output, err
}

func (c Client) do(
	ctx context.Context,
	method string,
	path string,
	headers map[string]string,
	input any,
	output any,
) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		method,
		strings.TrimRight(c.BaseURL, "/")+path,
		body,
	)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		var bodyValue map[string]any
		_ = json.Unmarshal(payload, &bodyValue)
		message := response.Status
		if value, ok := bodyValue["message"].(string); ok {
			message = value
		}
		return &Error{
			Status:  response.StatusCode,
			Message: message,
			Body:    bodyValue,
		}
	}
	if output == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, output)
}
