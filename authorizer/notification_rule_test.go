package authorizer_test

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/influxdata/influxdb/notification/rule"

	"github.com/google/go-cmp/cmp"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/authorizer"
	influxdbcontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/mock"
	influxdbtesting "github.com/influxdata/influxdb/testing"
)

var notificationRuleCmpOptions = cmp.Options{
	cmp.Comparer(func(x, y []byte) bool {
		return bytes.Equal(x, y)
	}),
	cmp.Transformer("Sort", func(in []influxdb.NotificationRule) []influxdb.NotificationRule {
		out := append([]influxdb.NotificationRule(nil), in...) // Copy input to avoid mutating it
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetID().String() > out[j].GetID().String()
		})
		return out
	}),
}

func TestNotificationRuleStore_FindNotificationRuleByID(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		permission influxdb.Permission
		id         influxdb.ID
	}
	type wants struct {
		err error
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to access id",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctx context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    id,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				permission: influxdb.Permission{
					Action: "read",
					Resource: influxdb.Resource{
						Type: influxdb.OrgsResourceType,
						ID:   influxdbtesting.IDPtr(10),
					},
				},
				id: 1,
			},
			wants: wants{
				err: nil,
			},
		},
		{
			name: "unauthorized to access id",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctx context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    id,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				permission: influxdb.Permission{
					Action: "read",
					Resource: influxdb.Resource{
						Type: influxdb.OrgsResourceType,
						ID:   influxdbtesting.IDPtr(2),
					},
				},
				id: 1,
			},
			wants: wants{
				err: &influxdb.Error{
					Msg:  "read:orgs/000000000000000a is unauthorized",
					Code: influxdb.EUnauthorized,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			_, err := s.FindNotificationRuleByID(ctx, tt.args.id)
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestNotificationRuleStore_FindNotificationRules(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		permission influxdb.Permission
	}
	type wants struct {
		err               error
		notificationRules []influxdb.NotificationRule
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to see all notificationRules",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRulesF: func(ctx context.Context, filter influxdb.NotificationRuleFilter, opt ...influxdb.FindOptions) ([]influxdb.NotificationRule, int, error) {
						return []influxdb.NotificationRule{
							&rule.Slack{
								Base: rule.Base{
									ID:    1,
									OrgID: 10,
								},
							},
							&rule.Slack{
								Base: rule.Base{
									ID:    2,
									OrgID: 10,
								},
							},
							&rule.PagerDuty{
								Base: rule.Base{
									ID:    3,
									OrgID: 11,
								},
							},
						}, 3, nil
					},
				},
			},
			args: args{
				permission: influxdb.Permission{
					Action: "read",
					Resource: influxdb.Resource{
						Type: influxdb.OrgsResourceType,
					},
				},
			},
			wants: wants{
				notificationRules: []influxdb.NotificationRule{
					&rule.Slack{
						Base: rule.Base{
							ID:    1,
							OrgID: 10,
						},
					},
					&rule.Slack{
						Base: rule.Base{
							ID:    2,
							OrgID: 10,
						},
					},
					&rule.PagerDuty{
						Base: rule.Base{
							ID:    3,
							OrgID: 11,
						},
					},
				},
			},
		},
		{
			name: "authorized to access a single orgs notificationRules",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRulesF: func(ctx context.Context, filter influxdb.NotificationRuleFilter, opt ...influxdb.FindOptions) ([]influxdb.NotificationRule, int, error) {
						return []influxdb.NotificationRule{
							&rule.Slack{
								Base: rule.Base{
									ID:    1,
									OrgID: 10,
								},
							},
							&rule.Slack{
								Base: rule.Base{
									ID:    2,
									OrgID: 10,
								},
							},
							&rule.PagerDuty{
								Base: rule.Base{
									ID:    3,
									OrgID: 11,
								},
							},
						}, 3, nil
					},
				},
			},
			args: args{
				permission: influxdb.Permission{
					Action: "read",
					Resource: influxdb.Resource{
						Type:  influxdb.OrgsResourceType,
						OrgID: influxdbtesting.IDPtr(10),
					},
				},
			},
			wants: wants{
				notificationRules: []influxdb.NotificationRule{
					&rule.Slack{
						Base: rule.Base{
							ID:    1,
							OrgID: 10,
						},
					},
					&rule.Slack{
						Base: rule.Base{
							ID:    2,
							OrgID: 10,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			ts, _, err := s.FindNotificationRules(ctx, influxdb.NotificationRuleFilter{})
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)

			if diff := cmp.Diff(ts, tt.wants.notificationRules, notificationRuleCmpOptions...); diff != "" {
				t.Errorf("notificationRules are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func TestNotificationRuleStore_UpdateNotificationRule(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		id          influxdb.ID
		permissions []influxdb.Permission
	}
	type wants struct {
		err error
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to update notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					UpdateNotificationRuleF: func(ctx context.Context, id influxdb.ID, upd influxdb.NotificationRule, userID influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "write",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
				},
			},
			wants: wants{
				err: nil,
			},
		},
		{
			name: "unauthorized to update notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					UpdateNotificationRuleF: func(ctx context.Context, id influxdb.ID, upd influxdb.NotificationRule, userID influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
				},
			},
			wants: wants{
				err: &influxdb.Error{
					Msg:  "write:orgs/000000000000000a is unauthorized",
					Code: influxdb.EUnauthorized,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			_, err := s.UpdateNotificationRule(ctx, tt.args.id, &rule.Slack{}, influxdb.ID(1))
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestNotificationRuleStore_PatchNotificationRule(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		id          influxdb.ID
		permissions []influxdb.Permission
	}
	type wants struct {
		err error
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to patch notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					PatchNotificationRuleF: func(ctx context.Context, id influxdb.ID, upd influxdb.NotificationRuleUpdate) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "write",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
				},
			},
			wants: wants{
				err: nil,
			},
		},
		{
			name: "unauthorized to patch notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					PatchNotificationRuleF: func(ctx context.Context, id influxdb.ID, upd influxdb.NotificationRuleUpdate) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(1),
						},
					},
				},
			},
			wants: wants{
				err: &influxdb.Error{
					Msg:  "write:orgs/000000000000000a is unauthorized",
					Code: influxdb.EUnauthorized,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			_, err := s.PatchNotificationRule(ctx, tt.args.id, influxdb.NotificationRuleUpdate{})
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestNotificationRuleStore_DeleteNotificationRule(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		id          influxdb.ID
		permissions []influxdb.Permission
	}
	type wants struct {
		err error
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to delete notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					DeleteNotificationRuleF: func(ctx context.Context, id influxdb.ID) error {
						return nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "write",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(10),
						},
					},
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(1),
						},
					},
				},
			},
			wants: wants{
				err: nil,
			},
		},
		{
			name: "unauthorized to delete notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					FindNotificationRuleByIDF: func(ctc context.Context, id influxdb.ID) (influxdb.NotificationRule, error) {
						return &rule.Slack{
							Base: rule.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					DeleteNotificationRuleF: func(ctx context.Context, id influxdb.ID) error {
						return nil
					},
				},
			},
			args: args{
				id: 1,
				permissions: []influxdb.Permission{
					{
						Action: "read",
						Resource: influxdb.Resource{
							Type: influxdb.OrgsResourceType,
							ID:   influxdbtesting.IDPtr(1),
						},
					},
				},
			},
			wants: wants{
				err: &influxdb.Error{
					Msg:  "write:orgs/000000000000000a is unauthorized",
					Code: influxdb.EUnauthorized,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			err := s.DeleteNotificationRule(ctx, tt.args.id)
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestNotificationRuleStore_CreateNotificationRule(t *testing.T) {
	type fields struct {
		NotificationRuleStore influxdb.NotificationRuleStore
	}
	type args struct {
		permission influxdb.Permission
		orgID      influxdb.ID
	}
	type wants struct {
		err error
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to create notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					CreateNotificationRuleF: func(ctx context.Context, tc influxdb.NotificationRule, userID influxdb.ID) error {
						return nil
					},
				},
			},
			args: args{
				orgID: 10,
				permission: influxdb.Permission{
					Action: "write",
					Resource: influxdb.Resource{
						Type:  influxdb.OrgsResourceType,
						OrgID: influxdbtesting.IDPtr(10),
					},
				},
			},
			wants: wants{
				err: nil,
			},
		},
		{
			name: "unauthorized to create notificationRule",
			fields: fields{
				NotificationRuleStore: &mock.NotificationRuleStore{
					CreateNotificationRuleF: func(ctx context.Context, tc influxdb.NotificationRule, userID influxdb.ID) error {
						return nil
					},
				},
			},
			args: args{
				orgID: 10,
				permission: influxdb.Permission{
					Action: "write",
					Resource: influxdb.Resource{
						Type: influxdb.OrgsResourceType,
						ID:   influxdbtesting.IDPtr(1),
					},
				},
			},
			wants: wants{
				err: &influxdb.Error{
					Msg:  "write:orgs/000000000000000a/orgs is unauthorized",
					Code: influxdb.EUnauthorized,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := authorizer.NewNotificationRuleStore(tt.fields.NotificationRuleStore, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			err := s.CreateNotificationRule(ctx, &rule.Slack{
				Base: rule.Base{
					OrgID: tt.args.orgID},
			}, influxdb.ID(1))
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}
