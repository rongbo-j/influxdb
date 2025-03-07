package rule

import (
	"encoding/json"

	"github.com/influxdata/influxdb"
)

// PagerDuty is the rule config of pagerduty notification.
type PagerDuty struct {
	Base
	MessageTemplate string `json:"messageTemplate"`
}

// GenerateFlux Generates the flux pager duty notification.
func (d *PagerDuty) GenerateFlux(e influxdb.NotificationEndpoint) (string, error) {
	// TODO(desa): needs implementation
	return `package main
data = from(bucket: "telegraf")
	|> range(start: -1m)

option task = {name: "name1", every: 1m}`, nil
}

type pagerDutyAlias PagerDuty

// MarshalJSON implement json.Marshaler interface.
func (c PagerDuty) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			pagerDutyAlias
			Type string `json:"type"`
		}{
			pagerDutyAlias: pagerDutyAlias(c),
			Type:           c.Type(),
		})
}

// Valid returns where the config is valid.
func (c PagerDuty) Valid() error {
	if err := c.Base.valid(); err != nil {
		return err
	}
	if c.MessageTemplate == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "pagerduty invalid message template",
		}
	}
	return nil
}

// Type returns the type of the rule config.
func (c PagerDuty) Type() string {
	return "pagerduty"
}
