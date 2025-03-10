package check

import (
	"encoding/json"
	"fmt"

	"github.com/influxdata/flux/ast"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/notification"
	"github.com/influxdata/influxdb/notification/flux"
)

// Base will embed inside a check.
type Base struct {
	ID                    influxdb.ID             `json:"id,omitempty"`
	Name                  string                  `json:"name"`
	Description           string                  `json:"description,omitempty"`
	OwnerID               influxdb.ID             `json:"ownerID,omitempty"`
	OrgID                 influxdb.ID             `json:"orgID,omitempty"`
	Status                influxdb.Status         `json:"status"`
	Query                 influxdb.DashboardQuery `json:"query"`
	StatusMessageTemplate string                  `json:"statusMessageTemplate"`

	// Care should be taken to prevent TaskID from being exposed publicly.
	TaskID influxdb.ID `json:"taskID,omitempty"`

	Cron  string                 `json:"cron,omitempty"`
	Every *notification.Duration `json:"every,omitempty"`
	// Offset represents a delay before execution.
	// It gets marshalled from a string duration, i.e.: "10s" is 10 seconds
	Offset *notification.Duration `json:"offset,omitempty"`

	Tags []notification.Tag `json:"tags"`
	influxdb.CRUDLog
}

// Valid returns err if the check is invalid.
func (b Base) Valid() error {
	if !b.ID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Check ID is invalid",
		}
	}
	if b.Name == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Check Name can't be empty",
		}
	}
	if !b.OwnerID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Check OwnerID is invalid",
		}
	}
	if !b.OrgID.Valid() {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "Check OrgID is invalid",
		}
	}
	if b.Status != influxdb.Active && b.Status != influxdb.Inactive {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "invalid status",
		}
	}
	for _, tag := range b.Tags {
		if err := tag.Valid(); err != nil {
			return err
		}
	}

	return nil
}

func (b Base) generateFluxASTMessageFunction() ast.Statement {
	fn := flux.Function(flux.FunctionParams("r"), flux.String(b.StatusMessageTemplate))
	return flux.DefineVariable("messageFn", fn)
}

func (b Base) generateTaskOption() ast.Statement {
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

func (b Base) generateFluxASTCheckDefinition(checkType string) ast.Statement {
	props := []*ast.Property{}
	props = append(props, flux.Property("_check_id", flux.String(b.ID.String())))
	props = append(props, flux.Property("_check_name", flux.String(b.Name)))
	props = append(props, flux.Property("_check_type", flux.String(checkType)))

	// TODO(desa): eventually tags will be flattened out into the data struct
	tagProps := []*ast.Property{}
	for _, tag := range b.Tags {
		tagProps = append(tagProps, flux.Property(tag.Key, flux.String(tag.Value)))
	}

	props = append(props, flux.Property("tags", flux.Object(tagProps...)))

	return flux.DefineVariable("check", flux.Object(props...))
}

// GetID implements influxdb.Getter interface.
func (b Base) GetID() influxdb.ID {
	return b.ID
}

// GetOrgID implements influxdb.Getter interface.
func (b Base) GetOrgID() influxdb.ID {
	return b.OrgID
}

// GetOwnerID gets the ownerID associated with a Base.
func (b Base) GetOwnerID() influxdb.ID {
	return b.OwnerID
}

// GetTaskID retrieves the task ID for a check.
func (b Base) GetTaskID() influxdb.ID {
	return b.TaskID
}

// GetCRUDLog implements influxdb.Getter interface.
func (b Base) GetCRUDLog() influxdb.CRUDLog {
	return b.CRUDLog
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

// ClearPrivateData remove any data that we don't want to be exposed publicly.
func (b *Base) ClearPrivateData() {
	b.TaskID = 0
}

// SetTaskID sets the taskID for a check.
func (b *Base) SetTaskID(id influxdb.ID) {
	b.TaskID = id
}

// SetOwnerID sets the taskID for a check.
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

var typeToCheck = map[string](func() influxdb.Check){
	"deadman":   func() influxdb.Check { return &Deadman{} },
	"threshold": func() influxdb.Check { return &Threshold{} },
}

type rawRuleJSON struct {
	Typ string `json:"type"`
}

// UnmarshalJSON will convert
func UnmarshalJSON(b []byte) (influxdb.Check, error) {
	var raw rawRuleJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, &influxdb.Error{
			Msg: "unable to detect the check type from json",
		}
	}
	convertedFunc, ok := typeToCheck[raw.Typ]
	if !ok {
		return nil, &influxdb.Error{
			Msg: fmt.Sprintf("invalid check type %s", raw.Typ),
		}
	}
	converted := convertedFunc()
	err := json.Unmarshal(b, converted)
	return converted, err
}
