package client

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AtellarClient struct {
	Token  *string `json:"token,omitempty"`
	Secret *string `json:"secret,omitempty"`
}

type AtellarClientCreationOptions struct {
	Token  *string `json:"token,omitempty"`
	Secret *string `json:"secret,omitempty"`
}

func NewAtellarClient(opts AtellarClientCreationOptions) AtellarClient {
	resolvedToken := opts.Token
	resolvedSecret := opts.Secret

	if resolvedSecret == nil {
		osSecret, err := os.ReadFile("/var/run/secrets/atellar/service-account/secret")
		if err == nil {
			secret := string(osSecret)
			resolvedSecret = &secret
		} else {
			envSecret := os.Getenv("ATELLAR_SERVICE_ACCOUNT_SECRET")
			if envSecret != "" {
				resolvedSecret = &envSecret
			}
		}
	}

	if resolvedToken == nil {
		envToken := os.Getenv("ATELLAR_SERVICE_ACCOUNT_TOKEN")
		if envToken != "" {
			resolvedToken = &envToken
		}
	}

	return AtellarClient{
		Token:  resolvedToken,
		Secret: resolvedSecret,
	}
}

func (c *AtellarClient) EnsureToken() error {
	if c.Token != nil {
		token, err := jwt.Parse(*c.Token, func(*jwt.Token) (any, error) {
			return *c.Secret, nil
		})

		if err != nil {
			t, err := token.Claims.GetExpirationTime()
			if err != nil {
				// if there is error. It must be undefined expiration date, su we force to use expiration
				c.Token = nil
			} else if t.Before(time.Now().Add(time.Second * 3)) {
				c.Token = nil
			} else {
				c.Token = &token.Raw
			}
		}

		if c.Token != nil {
			return nil
		}
	}

	if c.Secret == nil {
		return errors.New("Secret required for token generation")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})

	signed, err := jwt.SigningMethodHS256.Sign(*c.Secret, token.Raw)
	if err != nil {
		return err
	}

	signedString := string(signed)
	c.Token = &signedString
	return nil
}
