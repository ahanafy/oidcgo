package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ahanafy/oidcgo/pkg/oauth2dev"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func main() {
	var (
		clientID    = os.Getenv("OIDCCLIENTID")
		providerURL = os.Getenv("OIDCPROVIDERURL")
	)
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		log.Fatal(err)
	}

	// The usual OAuth2 configuration
	var clientOAuthConfig = &oauth2.Config{
		ClientID: clientID,
		Endpoint: provider.Endpoint(),

		// for example...
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Augment OAuth2 configuration with device endpoints.
	var clientDeviceOAuthConfig = &oauth2dev.Config{
		Config: clientOAuthConfig,
		DeviceEndpoint: oauth2dev.DeviceEndpoint{
			CodeURL: provider.Endpoint().AuthURL + "/device",
		},
	}

	// Use default HTTP client.
	client := http.DefaultClient

	// Get URL and code for user.
	dcr, err := oauth2dev.RequestDeviceCode(client, clientDeviceOAuthConfig)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Visit: %v\n", dcr.VerificationURLComplete)

	// Wait for a token. It will be a standard oauth2.Token.
	accessToken, err := oauth2dev.WaitForDeviceAuthorization(client,
		clientDeviceOAuthConfig, dcr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Access token: %v\n", accessToken)

	// Now use the token as usual...
}
