package rule

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/influxdata/flux/ast"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/notification"
	"github.com/influxdata/influxdb/notification/flux"
)

var typeToRule = map[string](func() influxdb.NotificationRule){
	"slack":     func() influxdb.NotificationRule { return &Slack{} },
	"pagerduty": func() influxdb.NotificationRule { return &PagerDuty{} },
	"http":      func() influxdb.NotificationRule { return &HTTP{} },
}

type rawRuleJSON struct {
	Typ string `json:"type"`
}

// UnmarshalJSON will convert
func UnmarshalJSON(b []byte) (influxdb.NotificationRule, error) {
	var raw rawRuleJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, &influxdb.Error{
			Msg: "unable to detect the notification type from json",
		}
	}
	convertedFunc, ok := typeToRule[raw.Typ]
	if !ok {
		return nil, &influxdb.Error{
			Msg: fmt.Sprintf("invalid notification type %s", raw.Typ),
		}
	}
	converted := convertedFunc()
	err := json.Unmarshal(b, converted)
	return converted, err
}

// Base is the embed struct of every notification rule.
type Base struct {
	ID          influxdb.ID     `json:"id,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	EndpointID  influxdb.ID     `json:"endpointID,omitempty"`
	OrgID       influxdb.ID     `json:"orgID,omitempty"`
	OwnerID     influxdb.ID     `json:"ownerID,omitempty"`
	TaskID      influxdb.ID     `json:"taskID,omitempty"`
	Status      influxdb.Status `json:"status"`
	// SleepUntil is an optional sleeptime to start a task.
	SleepUntil *time.Time             `json:"sleepUntil,omitempty"`
	Cron       string                 `json:"cron,omitempty"`
	Every      *notification.Duration `json:"every,omitempty"`
	// Offset represents a delay before execution.
	// It gets marshalled from a string duration, i.e.: "10s" is 10 seconds
	Offset      *notification.Duration    `json:"offset,omitempty"`
	RunbookLink string                    `json:"runbookLink"`
	TagRules    []notification.TagRule    `json:"tagRules,omitempty"`
	StatusRules []notification.StatusRule `json:"statusRules,omitempty"`
	*influxdb.Limit
	influxdb.CRUDLog
}

func (b Base) valid() error {
	if !b.ID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Notification Rule ID is invalid",
		}
	}
	if b.Name == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Notification Rule Name can't be empty",
		}
	}
	if !b.OwnerID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Notification Rule OwnerID is invalid",
		}
	}
	if !b.OrgID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Notification Rule OrgID is invalid",
		}
	}
	if !b.EndpointID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Notification Rule EndpointID is invalid",
		}
	}
	if b.Status != influxdb.Active && b.Status != influxdb.Inactive {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid status",
		}
	}
	for _, tagRule := range b.TagRules {
		if err := tagRule.Valid(); err != nil {
			return err
		}
	}
	if b.Limit != nil {
		if b.Limit.Every <= 0 || b.Limit.Rate <= 0 {
			return &influxdb.Error{
				Code: influxdb.EInvalid,
				Msg:  "if limit is set, limit and limitEvery must be larger than 0",
			}
		}
	}

	return nil
}
func (b *Base) generateFluxASTNotificationDefinition(e influxdb.NotificationEndpoint) ast.Statement {
	ruleID := flux.Property("_notification_rule_id", flux.String(b.ID.String()))
	ruleName := flux.Property("_notification_rule_name", flux.String(b.Name))
	endpointID := flux.Property("_notification_endpoint_id", flux.String(b.EndpointID.String()))
	endpointName := flux.Property("_notification_endpoint_name", flux.String(e.GetName()))

	return flux.DefineVariable("notification", flux.Object(ruleID, ruleName, endpointID, endpointName))
}

func (b *Base) generateTaskOption() ast.Statement {
	props := []*ast.Property{}

	props = append(props, flux.Property("name", flux.String(b.Name)))

	if b.Cron != "" {
		props = append(props, flux.Property("cron", flux.String(b.Cron)))
	}

	if b.Every != nil {
		props = append(props, flux.Property("every", (*ast.DurationLiteral)(b.Every)))
	}

	if b.Offset != nil {
		props = append(props, flux.Property("offset", (*ast.DurationLiteral)(b.Offset)))
	}

	return flux.DefineTaskOption(flux.Object(props...))
}

func (b *Base) generateFluxASTStatuses() ast.Statement {
	props := []*ast.Property{}

	props = append(props, flux.Property("start", flux.Negative((*ast.DurationLiteral)(b.Every))))

	if len(b.TagRules) > 0 {
		r := b.TagRules[0]
		var body ast.Expression = r.GenerateFluxAST()
		for _, r := range b.TagRules[1:] {
			body = flux.And(body, r.GenerateFluxAST())
		}
		props = append(props, flux.Property("fn", flux.Function(flux.FunctionParams("r"), body)))
	}

	base := flux.Call(flux.Member("monitor", "from"), flux.Object(props...))

	return flux.DefineVariable("statuses", base)
}

// GetID implements influxdb.Getter interface.
func (b Base) GetID() influxdb.ID {
	return b.ID
}

// GetEndpointID gets the endpointID for a base.
func (b Base) GetEndpointID() influxdb.ID {
	return b.EndpointID
}

// GetOrgID implements influxdb.Getter interface.
func (b Base) GetOrgID() influxdb.ID {
	return b.OrgID
}

// GetTaskID gets the task ID for a base.
func (b Base) GetTaskID() influxdb.ID {
	return b.TaskID
}

// SetTaskID sets the task ID for a base.
func (b *Base) SetTaskID(id influxdb.ID) {
	b.TaskID = id
}

// Clears the task ID from the base.
func (b *Base) ClearPrivateData() {
	b.TaskID = 0
}

// GetOwnerID returns the owner id.
func (b Base) GetOwnerID() influxdb.ID {
	return b.OwnerID
}

// GetCRUDLog implements influxdb.Getter interface.
func (b Base) GetCRUDLog() influxdb.CRUDLog {
	return b.CRUDLog
}

// GetLimit returns the limit pointer.
func (b *Base) GetLimit() *influxdb.Limit {
	return b.Limit
}

// GetName implements influxdb.Getter interface.
func (b *Base) GetName() string {
	return b.Name
}

// GetDescription implements influxdb.Getter interface.
func (b *Base) GetDescription() string {
	return b.Description
}

// GetStatus implements influxdb.Getter interface.
func (b *Base) GetStatus() influxdb.Status {
	return b.Status
}

// SetID will set the primary key.
func (b *Base) SetID(id influxdb.ID) {
	b.ID = id
}

// SetOrgID will set the org key.
func (b *Base) SetOrgID(id influxdb.ID) {
	b.OrgID = id
}

// SetOwnerID will set the owner id.
func (b *Base) SetOwnerID(id influxdb.ID) {
	b.OwnerID = id
}

// SetName implements influxdb.Updator interface.
func (b *Base) SetName(name string) {
	b.Name = name
}

// SetDescription implements influxdb.Updator interface.
func (b *Base) SetDescription(description string) {
	b.Description = description
}

// SetStatus implements influxdb.Updator interface.
func (b *Base) SetStatus(status influxdb.Status) {
	b.Status = status
}
