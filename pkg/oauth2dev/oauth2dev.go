package oauth2dev

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// A DeviceCode represents the user-visible code, verification URL and
// device-visible code used to allow for user authorisation of this app. The
// app should show UserCode and VerificationURL to the user.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURL         string `json:"verification_uri"`
	VerificationURLComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

// DeviceEndpoint contains the URLs required to initiate the OAuth2.0 flow for a
// provider's device flow.
type DeviceEndpoint struct {
	CodeURL string
}

// A version of oauth2.Config augmented with device endpoints
type Config struct {
	*oauth2.Config
	DeviceEndpoint DeviceEndpoint
}

// A tokenOrError is either an OAuth2 Token response or an error indicating why
// such a response failed.
type tokenOrError struct {
	*oauth2.Token
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	ExpiresIn        int64  `json:"expires_in"`
}

var (
	// ErrAccessDenied is an error returned when the user has denied this
	// app access to their account.
	ErrAccessDenied = errors.New("access denied by user")
)

const (
	deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
)

// RequestDeviceCode will initiate the OAuth2 device authorization flow. It
// requests a device code and information on the code and URL to show to the
// user. Pass the returned DeviceCode to WaitForDeviceAuthorization.
func RequestDeviceCode(client *http.Client, config *Config) (*DeviceCode, error) {
	scopes := strings.Join(config.Scopes, " ")
	resp, err := client.PostForm(config.DeviceEndpoint.CodeURL,
		url.Values{"client_id": {config.ClientID}, "scope": {scopes}})

	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"request for device code authorisation returned status %v (%v)",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Unmarshal response
	var dcr DeviceCode
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&dcr); err != nil {
		return nil, err
	}

	return &dcr, nil
}

// WaitForDeviceAuthorization polls the token URL waiting for the user to
// authorize the app. Upon authorization, it returns the new token. If
// authorization fails then an error is returned. If that failure was due to a
// user explicitly denying access, the error is ErrAccessDenied.
func WaitForDeviceAuthorization(client *http.Client, config *Config, code *DeviceCode) (*oauth2.Token, error) {
	for {
		resp, err := client.PostForm(config.Endpoint.TokenURL,
			url.Values{
				"client_secret": {config.ClientSecret},
				"client_id":     {config.ClientID},
				"device_code":   {code.DeviceCode},
				"grant_type":    {deviceGrantType}})
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusPreconditionRequired {
			time.Sleep(time.Duration(code.Interval) * time.Second)
			continue

		} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
			return nil, fmt.Errorf("HTTP error %v (%v) when polling for OAuth token",
				resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		// Unmarshal response, checking for errors
		var token tokenOrError
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&token); err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusBadRequest {
			if token.Expiry.IsZero() && token.ExpiresIn != 0 {
				token.Expiry = time.Now().Add(time.Duration(token.ExpiresIn * int64(time.Second)))
			}
		}

		switch token.Error {
		case "":

			return token.Token, nil
		case "authorization_pending":

		case "slow_down":

			code.Interval *= 2
		case "access_denied":

			return nil, ErrAccessDenied
		default:

			return nil, fmt.Errorf("authorization failed: %v", token.Error)
		}

		time.Sleep(time.Duration(code.Interval) * time.Second)
	}
}
