package check_test

import (
	"testing"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/notification"
	"github.com/influxdata/influxdb/notification/check"
)

func TestDeadman_GenerateFlux(t *testing.T) {
	type args struct {
		deadman check.Deadman
	}
	type wants struct {
		script string
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name: "basic",
			args: args{
				deadman: check.Deadman{
					Base: check.Base{
						ID:   10,
						Name: "moo",
						Tags: []notification.Tag{
							{Key: "aaa", Value: "vaaa"},
							{Key: "bbb", Value: "vbbb"},
						},
						Every:                 mustDuration("1h"),
						StatusMessageTemplate: "whoa! {r.dead}",
						Query: influxdb.DashboardQuery{
							Text: `from(bucket: "foo") |> range(start: -1d, stop: now()) |> aggregateWindow(every: 1m, fn: mean) |> yield()`,
							BuilderConfig: influxdb.BuilderConfig{
								Tags: []struct {
									Key    string   `json:"key"`
									Values []string `json:"values"`
								}{
									{
										Key:    "_field",
										Values: []string{"usage_user"},
									},
								},
							},
						},
					},
					TimeSince: 60,
					Level:     notification.Info,
				},
			},
			wants: wants{
				script: `package main
import "influxdata/influxdb/monitor"
import "experimental"

data = from(bucket: "foo")
	|> range(start: -1h)
	|> aggregateWindow(every: 1h, fn: mean)

option task = {name: "moo", every: 1h}

check = {
	_check_id: "000000000000000a",
	_check_name: "moo",
	_check_type: "deadman",
	tags: {aaa: "vaaa", bbb: "vbbb"},
}
info = (r) =>
	(r.dead)
messageFn = (r) =>
	("whoa! {r.dead}")

data
	|> monitor.deadman(t: experimental.subDuration(from: now(), d: 60s))
	|> monitor.check(data: check, messageFn: messageFn, info: info)`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := tt.args.deadman.GenerateFlux()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if exp, got := tt.wants.script, s; exp != got {
				t.Errorf("expected:\n%v\n\ngot:\n%v\n", exp, got)
			}
		})
	}

}
