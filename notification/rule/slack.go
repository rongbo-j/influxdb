package rule

import (
	"encoding/json"
	"fmt"

	"github.com/influxdata/flux/ast"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/notification/endpoint"
	"github.com/influxdata/influxdb/notification/flux"
)

// Slack is the notification rule config of slack.
type Slack struct {
	Base
	Channel         string `json:"channel"`
	MessageTemplate string `json:"messageTemplate"`
}

// GenerateFlux generates a flux script for the slack notification rule.
func (s *Slack) GenerateFlux(e influxdb.NotificationEndpoint) (string, error) {
	slackEndpoint, ok := e.(*endpoint.Slack)
	if !ok {
		return "", fmt.Errorf("endpoint provided is a %s, not an Slack endpoint", e.Type())
	}
	p, err := s.GenerateFluxAST(slackEndpoint)
	if err != nil {
		return "", err
	}
	return ast.Format(p), nil
}

// GenerateFluxAST generates a flux AST for the slack notification rule.
func (s *Slack) GenerateFluxAST(e *endpoint.Slack) (*ast.Package, error) {
	f := flux.File(
		s.Name,
		flux.Imports("influxdata/influxdb/monitor", "slack", "influxdata/influxdb/secrets"),
		s.generateFluxASTBody(e),
	)
	return &ast.Package{Package: "main", Files: []*ast.File{f}}, nil
}

func (s *Slack) generateFluxASTBody(e *endpoint.Slack) []ast.Statement {
	var statements []ast.Statement
	statements = append(statements, s.generateTaskOption())
	statements = append(statements, s.generateFluxASTSecrets(e))
	statements = append(statements, s.generateFluxASTEndpoint(e))
	statements = append(statements, s.generateFluxASTNotificationDefinition(e))
	statements = append(statements, s.generateFluxASTStatuses())
	statements = append(statements, s.generateFluxASTNotifyPipe())

	return statements
}

func (s *Slack) generateFluxASTSecrets(e *endpoint.Slack) ast.Statement {
	call := flux.Call(flux.Member("secrets", "get"), flux.Object(flux.Property("key", flux.String(e.Token.Key))))

	return flux.DefineVariable("slack_secret", call)
}

func (s *Slack) generateFluxASTEndpoint(e *endpoint.Slack) ast.Statement {
	call := flux.Call(flux.Member("slack", "endpoint"),
		flux.Object(
			flux.Property("token", flux.Identifier("slack_secret")),
			flux.Property("url", flux.String(e.URL)),
		),
	)

	return flux.DefineVariable("slack_endpoint", call)
}

func (s *Slack) generateFluxASTNotifyPipe() ast.Statement {
	endpointProps := []*ast.Property{}
	endpointProps = append(endpointProps, flux.Property("channel", flux.String(s.Channel)))
	// TODO(desa): are these values correct?
	endpointProps = append(endpointProps, flux.Property("text", flux.String(s.MessageTemplate)))
	endpointFn := flux.Function(flux.FunctionParams("r"), flux.Object(endpointProps...))

	props := []*ast.Property{}
	props = append(props, flux.Property("data", flux.Identifier("notification")))
	props = append(props, flux.Property("endpoint",
		flux.Call(flux.Identifier("slack_endpoint"), flux.Object(flux.Property("mapFn", endpointFn)))))

	call := flux.Call(flux.Member("monitor", "notify"), flux.Object(props...))

	return flux.ExpressionStatement(flux.Pipe(flux.Identifier("statuses"), call))
}

type slackAlias Slack

// MarshalJSON implement json.Marshaler interface.
func (c Slack) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		struct {
			slackAlias
			Type string `json:"type"`
		}{
			slackAlias: slackAlias(c),
			Type:       c.Type(),
		})
}

// Valid returns where the config is valid.
func (c Slack) Valid() error {
	if err := c.Base.valid(); err != nil {
		return err
	}
	if c.MessageTemplate == "" {
		return &influxdb.Error{
			Code: influxdb.EInvalid,
			Msg:  "slack msg template is empty",
		}
	}
	return nil
}

// Type returns the type of the rule config.
func (c Slack) Type() string {
	return "slack"
}
