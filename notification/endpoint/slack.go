package endpoint

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/influxdata/influxdb"
)

var _ influxdb.NotificationEndpoint = &Slack{}

const slackTokenSuffix = "-token"

// Slack is the notification endpoint config of slack.
type Slack struct {
	Base
	// URL is a valid slack webhook URL
	// TODO(jm): validate this in unmarshaler
	// example: https://slack.com/api/chat.postMessage
	URL string `json:"url"`
	// Token is the bearer token for authorization
	Token influxdb.SecretField `json:"token"`
}

// BackfillSecretKeys fill back fill the secret field key during the unmarshalling
// if value of that secret field is not nil.
func (s *Slack) BackfillSecretKeys() {
	if s.Token.Key == "" && s.Token.Value != nil {
		s.Token.Key = s.ID.String() + slackTokenSuffix
	}
}

// SecretFields return available secret fields.
func (s Slack) SecretFields() []influxdb.SecretField {
	return []influxdb.SecretField{
		s.Token,
	}
}

// Valid returns error if some configuration is invalid
func (s Slack) Valid() error {
	if err := s.Base.valid(); err != nil {
		return err
	}
	if s.URL == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "slack endpoint URL is empty",
		}
	}
	if _, err := url.Parse(s.URL); err != nil {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  fmt.Sprintf("slack endpoint URL is invalid: %s", err.Error()),
		}
	}
	// TODO(desa): this requirement seems odd
	if s.Token.Key != s.ID.String()+slackTokenSuffix {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "slack endpoint token is invalid",
		}
	}
	return nil
}

type slackAlias Slack

// MarshalJSON implement json.Marshaler interface.
func (s Slack) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			slackAlias
			Type string `json:"type"`
		}{
			slackAlias: slackAlias(s),
			Type:       s.Type(),
		})
}

// Type returns the type.
func (s Slack) Type() string {
	return SlackType
}
