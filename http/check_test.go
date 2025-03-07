package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/influxdata/flux/parser"
	pcontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/notification"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/mock"
	"github.com/influxdata/influxdb/notification/check"
	influxTesting "github.com/influxdata/influxdb/testing"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
)

// NewMockCheckBackend returns a CheckBackend with mock services.
func NewMockCheckBackend() *CheckBackend {
	return &CheckBackend{
		Logger: zap.NewNop().With(zap.String("handler", "check")),

		CheckService:               mock.NewCheckService(),
		UserResourceMappingService: mock.NewUserResourceMappingService(),
		LabelService:               mock.NewLabelService(),
		UserService:                mock.NewUserService(),
		OrganizationService:        mock.NewOrganizationService(),
	}
}

func TestService_handleGetChecks(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
		LabelService influxdb.LabelService
	}
	type args struct {
		queryParams map[string][]string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	fl1 := 100.32
	fl2 := 200.64
	fl4 := 100.1
	fl5 := 3023.2

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "get all checks",
			fields: fields{
				&mock.CheckService{
					FindChecksFn: func(ctx context.Context, filter influxdb.CheckFilter, opts ...influxdb.FindOptions) ([]influxdb.Check, int, error) {
						return []influxdb.Check{
							&check.Deadman{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("0b501e7e557ab1ed"),
									Name:   "hello",
									OrgID:  influxTesting.MustIDBase16("50f7ba1150f7ba11"),
									Status: influxdb.Active,
									TaskID: 3,
								},
								Level: notification.Info,
							},
							&check.Threshold{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("c0175f0077a77005"),
									Name:   "example",
									OrgID:  influxTesting.MustIDBase16("7e55e118dbabb1ed"),
									Status: influxdb.Inactive,
									TaskID: 3,
								},
								Thresholds: []check.ThresholdConfig{
									&check.Greater{
										Value:               fl1,
										ThresholdConfigBase: check.ThresholdConfigBase{Level: notification.Critical},
									},
									&check.Lesser{
										Value:               fl2,
										ThresholdConfigBase: check.ThresholdConfigBase{Level: notification.Info},
									},
									&check.Range{Min: fl4, Max: fl5, Within: true},
								},
							},
						}, 2, nil
					},
				},
				&mock.LabelService{
					FindResourceLabelsFn: func(ctx context.Context, f influxdb.LabelMappingFilter) ([]*influxdb.Label, error) {
						labels := []*influxdb.Label{
							{
								ID:   influxTesting.MustIDBase16("fc3dc670a4be9b9a"),
								Name: "label",
								Properties: map[string]string{
									"color": "fff000",
								},
							},
						}
						return labels, nil
					},
				},
			},
			args: args{
				map[string][]string{
					"limit": {"1"},
				},
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "self": "/api/v2/checks?descending=false&limit=1&offset=0",
    "next": "/api/v2/checks?descending=false&limit=1&offset=1"
  },
  "checks": [
    {
      "links": {
        "self": "/api/v2/checks/0b501e7e557ab1ed",
        "labels": "/api/v2/checks/0b501e7e557ab1ed/labels",
        "owners": "/api/v2/checks/0b501e7e557ab1ed/owners",
        "members": "/api/v2/checks/0b501e7e557ab1ed/members"
      },
	  "createdAt": "0001-01-01T00:00:00Z",
	  "updatedAt": "0001-01-01T00:00:00Z",
      "id": "0b501e7e557ab1ed",
	  "orgID": "50f7ba1150f7ba11",
	  "name": "hello",
	  "level": "INFO",
	  "query": {
	    "builderConfig": {
	      "aggregateWindow": {
	        "period": ""
	      },
	      "buckets": null,
	      "functions": null,
	      "tags": null
	    },
	    "editMode": "",
	    "name": "",
	    "text": ""
	  },
	  "reportZero": false,
	  "status": "active",
	  "statusMessageTemplate": "",
	  "tags": null,
	  "timeSince": 0,
	  "type": "deadman",
      "labels": [
        {
          "id": "fc3dc670a4be9b9a",
          "name": "label",
          "properties": {
            "color": "fff000"
          }
        }
      ]
    },
    {
      "links": {
        "self": "/api/v2/checks/c0175f0077a77005",
        "labels": "/api/v2/checks/c0175f0077a77005/labels",
        "members": "/api/v2/checks/c0175f0077a77005/members",
        "owners": "/api/v2/checks/c0175f0077a77005/owners"
      },
	  "createdAt": "0001-01-01T00:00:00Z",
	  "updatedAt": "0001-01-01T00:00:00Z",
      "id": "c0175f0077a77005",
      "orgID": "7e55e118dbabb1ed",
      "name": "example",
	  "query": {
	  "builderConfig": {
	    "aggregateWindow": {
	      "period": ""
	    },
	    "buckets": null,
	    "functions": null,
	    "tags": null
	  },
	  "editMode": "",
	  "name": "",
	  "text": ""
	},
	"status": "inactive",
	"statusMessageTemplate": "",
	"tags": null,
	"thresholds": [
	  {
	    "allValues": false,
		"level": "CRIT",
		"type": "greater",
	    "value": 100.32
	  },
	  {
	    "allValues": false,
		"level": "INFO",
		"type": "lesser",
	    "value": 200.64
	  },
	  {
        "allValues": false,
        "level": "UNKNOWN",
        "max": 3023.2,
        "min": 100.1,
        "type": "range",
        "within": true
      }
	],
	"type": "threshold",
	"labels": [
        {
          "id": "fc3dc670a4be9b9a",
          "name": "label",
          "properties": {
            "color": "fff000"
          }
        }
      ]
    }
  ]
}
`,
			},
		},
		{
			name: "get all checks when there are none",
			fields: fields{
				&mock.CheckService{
					FindChecksFn: func(ctx context.Context, filter influxdb.CheckFilter, opts ...influxdb.FindOptions) ([]influxdb.Check, int, error) {
						return []influxdb.Check{}, 0, nil
					},
				},
				&mock.LabelService{},
			},
			args: args{
				map[string][]string{
					"limit": {"1"},
				},
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "self": "/api/v2/checks?descending=false&limit=1&offset=0"
  },
  "checks": []
}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.CheckService = tt.fields.CheckService
			checkBackend.LabelService = tt.fields.LabelService
			h := NewCheckHandler(checkBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			qp := r.URL.Query()
			for k, vs := range tt.args.queryParams {
				for _, v := range vs {
					qp.Add(k, v)
				}
			}
			r.URL.RawQuery = qp.Encode()

			w := httptest.NewRecorder()

			h.handleGetChecks(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleGetChecks() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleGetChecks() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil || tt.wants.body != "" && !eq {
				t.Errorf("%q. handleGetChecks() = ***%v***", tt.name, diff)
			}
		})
	}
}

func mustDuration(d string) *notification.Duration {
	dur, err := parser.ParseDuration(d)
	if err != nil {
		panic(err)
	}

	return (*notification.Duration)(dur)
}

func TestService_handleGetCheckQuery(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	var l float64 = 10
	var u float64 = 40
	type args struct {
		id string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "get a check query by id",
			fields: fields{
				&mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						if id == influxTesting.MustIDBase16("020f755c3c082000") {
							return &check.Threshold{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("020f755c3c082000"),
									OrgID:  influxTesting.MustIDBase16("020f755c3c082000"),
									Name:   "hello",
									Status: influxdb.Active,
									TaskID: 3,
									Tags: []notification.Tag{
										{Key: "aaa", Value: "vaaa"},
										{Key: "bbb", Value: "vbbb"},
									},
									Every:                 mustDuration("1h"),
									StatusMessageTemplate: "whoa! {check.yeah}",
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
								Thresholds: []check.ThresholdConfig{
									check.Greater{
										ThresholdConfigBase: check.ThresholdConfigBase{
											Level: notification.Ok,
										},
										Value: l,
									},
									check.Lesser{
										ThresholdConfigBase: check.ThresholdConfigBase{
											Level: notification.Info,
										},
										Value: u,
									},
									check.Range{
										ThresholdConfigBase: check.ThresholdConfigBase{
											Level: notification.Warn,
										},
										Min:    l,
										Max:    u,
										Within: true,
									},
									check.Range{
										ThresholdConfigBase: check.ThresholdConfigBase{
											Level: notification.Critical,
										},
										Min:    l,
										Max:    u,
										Within: true,
									},
								},
							}, nil
						}
						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body:        `{"flux":"package main\nimport \"influxdata/influxdb/monitor\"\nimport \"influxdata/influxdb/v1\"\n\ndata = from(bucket: \"foo\")\n\t|\u003e range(start: -1h)\n\t|\u003e aggregateWindow(every: 1h, fn: mean)\n\noption task = {name: \"hello\", every: 1h}\n\ncheck = {\n\t_check_id: \"020f755c3c082000\",\n\t_check_name: \"hello\",\n\t_check_type: \"threshold\",\n\ttags: {aaa: \"vaaa\", bbb: \"vbbb\"},\n}\nok = (r) =\u003e\n\t(r.usage_user \u003e 10.0)\ninfo = (r) =\u003e\n\t(r.usage_user \u003c 40.0)\nwarn = (r) =\u003e\n\t(r.usage_user \u003c 40.0 and r.usage_user \u003e 10.0)\ncrit = (r) =\u003e\n\t(r.usage_user \u003c 40.0 and r.usage_user \u003e 10.0)\nmessageFn = (r) =\u003e\n\t(\"whoa! {check.yeah}\")\n\ndata\n\t|\u003e v1.fieldsAsCols()\n\t|\u003e monitor.check(\n\t\tdata: check,\n\t\tmessageFn: messageFn,\n\t\tok: ok,\n\t\tinfo: info,\n\t\twarn: warn,\n\t\tcrit: crit,\n\t)"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.HTTPErrorHandler = ErrorHandler(0)
			checkBackend.CheckService = tt.fields.CheckService
			h := NewCheckHandler(checkBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handleGetCheckQuery(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleGetCheckQuery() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleGetCheckQuery() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil || tt.wants.body != "" && !eq {
				t.Errorf("%q. handleGetChecks() = ***%v***", tt.name, diff)
			}
		})
	}
}

func TestService_handleGetCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	type args struct {
		id string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "get a check by id",
			fields: fields{
				&mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						if id == influxTesting.MustIDBase16("020f755c3c082000") {
							return &check.Deadman{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("020f755c3c082000"),
									OrgID:  influxTesting.MustIDBase16("020f755c3c082000"),
									Name:   "hello",
									Status: influxdb.Active,
									Every:  mustDuration("3h"),
									TaskID: 3,
								},
								Level: notification.Critical,
							}, nil
						}
						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
		{
		  "links": {
		    "self": "/api/v2/checks/020f755c3c082000",
		    "labels": "/api/v2/checks/020f755c3c082000/labels",
		    "members": "/api/v2/checks/020f755c3c082000/members",
		    "owners": "/api/v2/checks/020f755c3c082000/owners"
		  },
		  "labels": [],
		  "level": "CRIT",
		  "every": "3h",
		  "createdAt": "0001-01-01T00:00:00Z",
		  "updatedAt": "0001-01-01T00:00:00Z",
		  "id": "020f755c3c082000",
		  "query": {
            "builderConfig": {
              "aggregateWindow": {
                "period": ""
              },
              "buckets": null,
              "functions": null,
              "tags": null
            },
            "editMode": "",
            "name": "",
            "text": ""
          },
          "reportZero": false,
          "status": "active",
          "statusMessageTemplate": "",
          "tags": null,
          "timeSince": 0,
          "type": "deadman",
		  "orgID": "020f755c3c082000",
		  "name": "hello"
		}
		`,
			},
		},
		{
			name: "not found",
			fields: fields{
				&mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return nil, &influxdb.Error{
							Code: influxdb.ENotFound,
							Msg:  "check not found",
						}
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.HTTPErrorHandler = ErrorHandler(0)
			checkBackend.CheckService = tt.fields.CheckService
			h := NewCheckHandler(checkBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handleGetCheck(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)
			t.Logf(res.Header.Get("X-Influx-Error"))

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleGetCheck() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleGetCheck() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handleGetCheck(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handleGetCheck() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePostCheck(t *testing.T) {
	type fields struct {
		CheckService        influxdb.CheckService
		OrganizationService influxdb.OrganizationService
	}
	type args struct {
		userID influxdb.ID
		check  influxdb.Check
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "create a new check",
			fields: fields{
				CheckService: &mock.CheckService{
					CreateCheckFn: func(ctx context.Context, c influxdb.Check, userID influxdb.ID) error {
						c.SetID(influxTesting.MustIDBase16("020f755c3c082000"))
						c.SetOwnerID(userID)
						return nil
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f influxdb.OrganizationFilter) (*influxdb.Organization, error) {
						return &influxdb.Organization{ID: influxTesting.MustIDBase16("6f626f7274697320")}, nil
					},
				},
			},
			args: args{
				userID: influxTesting.MustIDBase16("6f626f7274697321"),
				check: &check.Deadman{
					Base: check.Base{
						Name:                  "hello",
						OrgID:                 influxTesting.MustIDBase16("6f626f7274697320"),
						OwnerID:               influxTesting.MustIDBase16("6f626f7274697321"),
						Description:           "desc1",
						StatusMessageTemplate: "msg1",
						Status:                influxdb.Active,
						Every:                 mustDuration("5m"),
						TaskID:                3,
						Tags: []notification.Tag{
							{Key: "k1", Value: "v1"},
							{Key: "k2", Value: "v2"},
						},
					},
					TimeSince:  13,
					ReportZero: true,
					Level:      notification.Warn,
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "self": "/api/v2/checks/020f755c3c082000",
    "labels": "/api/v2/checks/020f755c3c082000/labels",
    "members": "/api/v2/checks/020f755c3c082000/members",
    "owners": "/api/v2/checks/020f755c3c082000/owners"
  },
  "reportZero": true,
  "status": "active",
  "statusMessageTemplate": "msg1",
  "tags": [
    {
      "key": "k1",
      "value": "v1"
    },
    {
      "key": "k2",
      "value": "v2"
    }
  ],
  "query": {
  	"builderConfig": {
    "aggregateWindow": {
      "period": ""
    },
    "buckets": null,
    "functions": null,
    "tags": null
  },
  "editMode": "",
  "name": "",
  "text": ""
},
  "type": "deadman",
  "timeSince": 13,
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "id": "020f755c3c082000",
  "orgID": "6f626f7274697320",
  "name": "hello",
  "ownerID": "6f626f7274697321",
  "description": "desc1",
  "every": "5m",
  "level": "WARN",
  "labels": []
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.CheckService = tt.fields.CheckService
			checkBackend.OrganizationService = tt.fields.OrganizationService
			h := NewCheckHandler(checkBackend)

			b, err := json.Marshal(tt.args.check)
			if err != nil {
				t.Fatalf("failed to unmarshal check: %v", err)
			}
			r := httptest.NewRequest("GET", "http://any.url?org=30", bytes.NewReader(b))
			w := httptest.NewRecorder()
			r = r.WithContext(pcontext.SetAuthorizer(r.Context(), &influxdb.Session{UserID: tt.args.userID}))

			h.handlePostCheck(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostCheck() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostCheck() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handlePostCheck(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handlePostCheck() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handleDeleteCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	type args struct {
		id string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "remove a check by id",
			fields: fields{
				&mock.CheckService{
					DeleteCheckFn: func(ctx context.Context, id influxdb.ID) error {
						if id == influxTesting.MustIDBase16("020f755c3c082000") {
							return nil
						}

						return fmt.Errorf("wrong id")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNoContent,
			},
		},
		{
			name: "check not found",
			fields: fields{
				&mock.CheckService{
					DeleteCheckFn: func(ctx context.Context, id influxdb.ID) error {
						return &influxdb.Error{
							Code: influxdb.ENotFound,
							Msg:  "check not found",
						}
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.HTTPErrorHandler = ErrorHandler(0)
			checkBackend.CheckService = tt.fields.CheckService
			h := NewCheckHandler(checkBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handleDeleteCheck(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleDeleteCheck() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleDeleteCheck() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handleDeleteCheck(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handleDeleteCheck() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePatchCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	type args struct {
		id   string
		name string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "update a check name",
			fields: fields{
				&mock.CheckService{
					PatchCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.CheckUpdate) (influxdb.Check, error) {
						if id == influxTesting.MustIDBase16("020f755c3c082000") {
							d := &check.Deadman{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("020f755c3c082000"),
									Name:   "hello",
									OrgID:  influxTesting.MustIDBase16("020f755c3c082000"),
									TaskID: 3,
								},
								Level: notification.Critical,
							}

							if upd.Name != nil {
								d.Name = *upd.Name
							}

							return d, nil
						}

						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id:   "020f755c3c082000",
				name: "example",
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
		{
		  "links": {
		    "self": "/api/v2/checks/020f755c3c082000",
		    "labels": "/api/v2/checks/020f755c3c082000/labels",
		    "members": "/api/v2/checks/020f755c3c082000/members",
		    "owners": "/api/v2/checks/020f755c3c082000/owners"
		  },
		  "createdAt": "0001-01-01T00:00:00Z",
		  "updatedAt": "0001-01-01T00:00:00Z",
		  "id": "020f755c3c082000",
		  "orgID": "020f755c3c082000",
		  "level": "CRIT",
		  "name": "example",
		  "query": {
            "builderConfig": {
              "aggregateWindow": {
                "period": ""
              },
              "buckets": null,
              "functions": null,
              "tags": null
            },
            "editMode": "",
            "name": "",
            "text": ""
          },
          "reportZero": false,
          "status": "",
          "statusMessageTemplate": "",
          "tags": null,
          "timeSince": 0,
          "type": "deadman",
		  "labels": []
		}
		`,
			},
		},
		{
			name: "check not found",
			fields: fields{
				&mock.CheckService{
					PatchCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.CheckUpdate) (influxdb.Check, error) {
						return nil, &influxdb.Error{
							Code: influxdb.ENotFound,
							Msg:  "check not found",
						}
					},
				},
			},
			args: args{
				id:   "020f755c3c082000",
				name: "hello",
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.HTTPErrorHandler = ErrorHandler(0)
			checkBackend.CheckService = tt.fields.CheckService
			h := NewCheckHandler(checkBackend)

			upd := influxdb.CheckUpdate{}
			if tt.args.name != "" {
				upd.Name = &tt.args.name
			}

			b, err := json.Marshal(upd)
			if err != nil {
				t.Fatalf("failed to unmarshal check update: %v", err)
			}

			r := httptest.NewRequest("GET", "http://any.url", bytes.NewReader(b))

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handlePatchCheck(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePatchCheck() = %v, want %v %v", tt.name, res.StatusCode, tt.wants.statusCode, w.Header())
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePatchCheck() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handlePatchCheck(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handlePatchCheck() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handleUpdateCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	type args struct {
		id  string
		chk influxdb.Check
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "update a check name",
			fields: fields{
				CheckService: &mock.CheckService{
					UpdateCheckFn: func(ctx context.Context, id influxdb.ID, chk influxdb.Check) (influxdb.Check, error) {
						if id == influxTesting.MustIDBase16("020f755c3c082000") {
							d := &check.Deadman{
								Base: check.Base{
									ID:     influxTesting.MustIDBase16("020f755c3c082000"),
									Name:   "hello",
									Status: influxdb.Inactive,
									OrgID:  influxTesting.MustIDBase16("020f755c3c082000"),
									TaskID: 3,
								},
							}

							d = chk.(*check.Deadman)
							d.SetID(influxTesting.MustIDBase16("020f755c3c082000"))
							d.SetOrgID(influxTesting.MustIDBase16("020f755c3c082000"))

							return d, nil
						}

						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
				chk: &check.Deadman{
					Base: check.Base{
						Name:   "example",
						Status: influxdb.Active,
						TaskID: 3,
					},
					Level: notification.Critical,
				},
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
		{
		  "links": {
		    "self": "/api/v2/checks/020f755c3c082000",
		    "labels": "/api/v2/checks/020f755c3c082000/labels",
		    "members": "/api/v2/checks/020f755c3c082000/members",
		    "owners": "/api/v2/checks/020f755c3c082000/owners"
		  },
		  "createdAt": "0001-01-01T00:00:00Z",
		  "updatedAt": "0001-01-01T00:00:00Z",
		  "id": "020f755c3c082000",
		  "orgID": "020f755c3c082000",
		  "level": "CRIT",
		  "name": "example",
		  "query": {
            "builderConfig": {
              "aggregateWindow": {
                "period": ""
              },
              "buckets": null,
              "functions": null,
              "tags": null
            },
            "editMode": "",
            "name": "",
            "text": ""
          },
          "reportZero": false,
          "status": "active",
          "statusMessageTemplate": "",
          "tags": null,
          "timeSince": 0,
          "type": "deadman",
		  "labels": []
		}
		`,
			},
		},
		{
			name: "check not found",
			fields: fields{
				CheckService: &mock.CheckService{
					UpdateCheckFn: func(ctx context.Context, id influxdb.ID, chk influxdb.Check) (influxdb.Check, error) {
						return nil, &influxdb.Error{
							Code: influxdb.ENotFound,
							Msg:  "check not found",
						}
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
				chk: &check.Deadman{
					Base: check.Base{
						Name: "example",
					},
				},
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.HTTPErrorHandler = ErrorHandler(0)
			checkBackend.CheckService = tt.fields.CheckService
			h := NewCheckHandler(checkBackend)

			b, err := json.Marshal(tt.args.chk)
			if err != nil {
				t.Fatalf("failed to unmarshal check update: %v", err)
			}

			r := httptest.NewRequest("GET", "http://any.url", bytes.NewReader(b))

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handlePutCheck(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePutCheck() = %v, want %v %v", tt.name, res.StatusCode, tt.wants.statusCode, w.Header())
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePutCheck() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handlePutCheck(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handlePutCheck() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePostCheckMember(t *testing.T) {
	type fields struct {
		UserService influxdb.UserService
	}
	type args struct {
		checkID string
		user    *influxdb.User
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "add a check member",
			fields: fields{
				UserService: &mock.UserService{
					FindUserByIDFn: func(ctx context.Context, id influxdb.ID) (*influxdb.User, error) {
						return &influxdb.User{
							ID:   id,
							Name: "name",
						}, nil
					},
				},
			},
			args: args{
				checkID: "020f755c3c082000",
				user: &influxdb.User{
					ID: influxTesting.MustIDBase16("6f626f7274697320"),
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "logs": "/api/v2/users/6f626f7274697320/logs",
    "self": "/api/v2/users/6f626f7274697320"
  },
  "role": "member",
  "id": "6f626f7274697320",
  "name": "name"
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.UserService = tt.fields.UserService
			h := NewCheckHandler(checkBackend)

			b, err := json.Marshal(tt.args.user)
			if err != nil {
				t.Fatalf("failed to marshal user: %v", err)
			}

			path := fmt.Sprintf("/api/v2/checks/%s/members", tt.args.checkID)
			r := httptest.NewRequest("POST", path, bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostCheckMember() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostCheckMember() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
				t.Errorf("%q, handlePostCheckMember(). error unmarshaling json %v", tt.name, err)
			} else if tt.wants.body != "" && !eq {
				t.Errorf("%q. handlePostCheckMember() = ***%s***", tt.name, diff)
			}
		})
	}
}

func TestService_handlePostCheckOwner(t *testing.T) {
	type fields struct {
		UserService influxdb.UserService
	}
	type args struct {
		checkID string
		user    *influxdb.User
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "add a check owner",
			fields: fields{
				UserService: &mock.UserService{
					FindUserByIDFn: func(ctx context.Context, id influxdb.ID) (*influxdb.User, error) {
						return &influxdb.User{
							ID:   id,
							Name: "name",
						}, nil
					},
				},
			},
			args: args{
				checkID: "020f755c3c082000",
				user: &influxdb.User{
					ID: influxTesting.MustIDBase16("6f626f7274697320"),
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "logs": "/api/v2/users/6f626f7274697320/logs",
    "self": "/api/v2/users/6f626f7274697320"
  },
  "role": "owner",
  "id": "6f626f7274697320",
  "name": "name"
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkBackend := NewMockCheckBackend()
			checkBackend.UserService = tt.fields.UserService
			h := NewCheckHandler(checkBackend)

			b, err := json.Marshal(tt.args.user)
			if err != nil {
				t.Fatalf("failed to marshal user: %v", err)
			}

			path := fmt.Sprintf("/api/v2/checks/%s/owners", tt.args.checkID)
			r := httptest.NewRequest("POST", path, bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostCheckOwner() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostCheckOwner() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
				t.Errorf("%q, handlePostCheckOwner(). error unmarshaling json %v", tt.name, err)
			} else if tt.wants.body != "" && !eq {
				t.Errorf("%q. handlePostCheckOwner() = ***%s***", tt.name, diff)
			}
		})
	}
}

// func initCheckService(f influxTesting.CheckFields, t *testing.T) (influxdb.CheckService, string, func()) {
// 	svc := inmem.NewService()
// 	svc.IDGenerator = f.IDGenerator
// 	svc.TimeGenerator = f.TimeGenerator
// 	if f.TimeGenerator == nil {
// 		svc.TimeGenerator = influxdb.RealTimeGenerator{}
// 	}

// 	ctx := context.Background()
// 	for _, o := range f.Organizations {
// 		if err := svc.PutOrganization(ctx, o); err != nil {
// 			t.Fatalf("failed to populate organizations")
// 		}
// 	}
// 	for _, b := range f.Checks {
// 		if err := svc.PutCheck(ctx, b); err != nil {
// 			t.Fatalf("failed to populate checks")
// 		}
// 	}

// 	checkBackend := NewMockCheckBackend()
// 	checkBackend.HTTPErrorHandler = ErrorHandler(0)
// 	checkBackend.CheckService = svc
// 	checkBackend.OrganizationService = svc
// 	handler := NewCheckHandler(checkBackend)
// 	server := httptest.NewServer(handler)
// 	client := CheckService{
// 		Addr:     server.URL,
// 		OpPrefix: inmem.OpPrefix,
// 	}
// 	done := server.Close

// 	return &client, inmem.OpPrefix, done
// }

// func TestCheckService(t *testing.T) {
// 	influxTestingCheckService(initCheckService, t)
// }
