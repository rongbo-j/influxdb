package endpoint

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/influxdata/influxdb"
)

var _ influxdb.NotificationEndpoint = &HTTP{}

const (
	httpTokenSuffix    = "-token"
	httpUsernameSuffix = "-username"
	httpPasswordSuffix = "-password"
)

// HTTP is the notification endpoint config of http.
type HTTP struct {
	Base
	// Path is the API path of HTTP
	URL string `json:"url"`
	// Token is the bearer token for authorization
	Token           influxdb.SecretField `json:"token,omitempty"`
	Username        influxdb.SecretField `json:"username,omitempty"`
	Password        influxdb.SecretField `json:"password,omitempty"`
	AuthMethod      string               `json:"authMethod"`
	Method          string               `json:"method"`
	ContentTemplate string               `json:"contentTemplate"`
}

// BackfillSecretKeys fill back fill the secret field key during the unmarshalling
// if value of that secret field is not nil.
func (s *HTTP) BackfillSecretKeys() {
	if s.Token.Key == "" && s.Token.Value != nil {
		s.Token.Key = s.ID.String() + httpTokenSuffix
	}
	if s.Username.Key == "" && s.Username.Value != nil {
		s.Username.Key = s.ID.String() + httpUsernameSuffix
	}
	if s.Password.Key == "" && s.Password.Value != nil {
		s.Password.Key = s.ID.String() + httpPasswordSuffix
	}
}

// SecretFields return available secret fields.
func (s HTTP) SecretFields() []influxdb.SecretField {
	arr := make([]influxdb.SecretField, 0)
	if s.Token.Key != "" {
		arr = append(arr, s.Token)
	}
	if s.Username.Key != "" {
		arr = append(arr, s.Username)
	}
	if s.Password.Key != "" {
		arr = append(arr, s.Password)
	}
	return arr
}

var goodHTTPAuthMethod = map[string]bool{
	"none":   true,
	"basic":  true,
	"bearer": true,
}

var goodHTTPMethod = map[string]bool{
	http.MethodGet:  true,
	http.MethodPost: true,
	http.MethodPut:  true,
}

// Valid returns error if some configuration is invalid
func (s HTTP) Valid() error {
	if err := s.Base.valid(); err != nil {
		return err
	}
	if s.URL == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "http endpoint URL is empty",
		}
	}
	if _, err := url.Parse(s.URL); err != nil {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  fmt.Sprintf("http endpoint URL is invalid: %s", err.Error()),
		}
	}
	if !goodHTTPMethod[s.Method] {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid http http method",
		}
	}
	if !goodHTTPAuthMethod[s.AuthMethod] {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid http auth method",
		}
	}
	if s.AuthMethod == "basic" &&
		(s.Username.Key != s.ID.String()+httpUsernameSuffix ||
			s.Password.Key != s.ID.String()+httpPasswordSuffix) {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid http username/password for basic auth",
		}
	}
	if s.AuthMethod == "bearer" && s.Token.Key != httpTokenSuffix {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid http token for bearer auth",
		}
	}

	return nil
}

type httpAlias HTTP

// MarshalJSON implement json.Marshaler interface.
func (s HTTP) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			httpAlias
			Type string `json:"type"`
		}{
			httpAlias: httpAlias(s),
			Type:      s.Type(),
		})
}

// Type returns the type.
func (s HTTP) Type() string {
	return HTTPType
}

// ParseResponse will parse the http response from http.
func (s HTTP) ParseResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return &influxdb.Error{
			Msg: string(body),
		}
	}
	return nil
}
