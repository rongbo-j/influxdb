package authorizer_test

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/influxdata/influxdb/notification/check"

	"github.com/google/go-cmp/cmp"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/authorizer"
	influxdbcontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/mock"
	influxdbtesting "github.com/influxdata/influxdb/testing"
)

var checkCmpOptions = cmp.Options{
	cmp.Comparer(func(x, y []byte) bool {
		return bytes.Equal(x, y)
	}),
	cmp.Transformer("Sort", func(in []influxdb.Check) []influxdb.Check {
		out := append([]influxdb.Check(nil), in...) // Copy input to avoid mutating it
		sort.Slice(out, func(i, j int) bool {
			return out[i].GetID() > out[j].GetID()
		})
		return out
	}),
}

func TestCheckService_FindCheckByID(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
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
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			_, err := s.FindCheckByID(ctx, tt.args.id)
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestCheckService_FindChecks(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
	}
	type args struct {
		permission influxdb.Permission
	}
	type wants struct {
		err    error
		checks []influxdb.Check
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "authorized to see all checks",
			fields: fields{
				CheckService: &mock.CheckService{
					FindChecksFn: func(ctx context.Context, filter influxdb.CheckFilter, opt ...influxdb.FindOptions) ([]influxdb.Check, int, error) {
						return []influxdb.Check{
							&check.Deadman{
								Base: check.Base{
									ID:    1,
									OrgID: 10,
								},
							},
							&check.Deadman{
								Base: check.Base{
									ID:    2,
									OrgID: 10,
								},
							},
							&check.Threshold{
								Base: check.Base{
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
				checks: []influxdb.Check{
					&check.Deadman{
						Base: check.Base{
							ID:    1,
							OrgID: 10,
						},
					},
					&check.Deadman{
						Base: check.Base{
							ID:    2,
							OrgID: 10,
						},
					},
					&check.Threshold{
						Base: check.Base{
							ID:    3,
							OrgID: 11,
						},
					},
				},
			},
		},
		{
			name: "authorized to access a single orgs checks",
			fields: fields{
				CheckService: &mock.CheckService{
					FindChecksFn: func(ctx context.Context, filter influxdb.CheckFilter, opt ...influxdb.FindOptions) ([]influxdb.Check, int, error) {
						return []influxdb.Check{
							&check.Deadman{
								Base: check.Base{
									ID:    1,
									OrgID: 10,
								},
							},
							&check.Deadman{
								Base: check.Base{
									ID:    2,
									OrgID: 10,
								},
							},
							&check.Threshold{
								Base: check.Base{
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
				checks: []influxdb.Check{
					&check.Deadman{
						Base: check.Base{
							ID:    1,
							OrgID: 10,
						},
					},
					&check.Deadman{
						Base: check.Base{
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			ts, _, err := s.FindChecks(ctx, influxdb.CheckFilter{})
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)

			if diff := cmp.Diff(ts, tt.wants.checks, checkCmpOptions...); diff != "" {
				t.Errorf("checks are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func TestCheckService_UpdateCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
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
			name: "authorized to update check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					UpdateCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.Check) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
			name: "unauthorized to update check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					UpdateCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.Check) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			_, err := s.UpdateCheck(ctx, tt.args.id, &check.Deadman{})
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestCheckService_PatchCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
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
			name: "authorized to patch check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					PatchCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.CheckUpdate) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
			name: "unauthorized to patch check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					PatchCheckFn: func(ctx context.Context, id influxdb.ID, upd influxdb.CheckUpdate) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			_, err := s.PatchCheck(ctx, tt.args.id, influxdb.CheckUpdate{})
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestCheckService_DeleteCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
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
			name: "authorized to delete check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					DeleteCheckFn: func(ctx context.Context, id influxdb.ID) error {
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
			name: "unauthorized to delete check",
			fields: fields{
				CheckService: &mock.CheckService{
					FindCheckByIDFn: func(ctx context.Context, id influxdb.ID) (influxdb.Check, error) {
						return &check.Deadman{
							Base: check.Base{
								ID:    1,
								OrgID: 10,
							},
						}, nil
					},
					DeleteCheckFn: func(ctx context.Context, id influxdb.ID) error {
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{tt.args.permissions})

			err := s.DeleteCheck(ctx, tt.args.id)
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}

func TestCheckService_CreateCheck(t *testing.T) {
	type fields struct {
		CheckService influxdb.CheckService
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
			name: "authorized to create check with org owner",
			fields: fields{
				CheckService: &mock.CheckService{
					CreateCheckFn: func(ctx context.Context, chk influxdb.Check, userID influxdb.ID) error {
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
			name: "unauthorized to create check",
			fields: fields{
				CheckService: &mock.CheckService{
					CreateCheckFn: func(ctx context.Context, chk influxdb.Check, userID influxdb.ID) error {
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
			s := authorizer.NewCheckService(tt.fields.CheckService, mock.NewUserResourceMappingService(), mock.NewOrganizationService())

			ctx := context.Background()
			ctx = influxdbcontext.SetAuthorizer(ctx, &Authorizer{[]influxdb.Permission{tt.args.permission}})

			err := s.CreateCheck(ctx, &check.Deadman{
				Base: check.Base{
					OrgID: tt.args.orgID},
			}, 3)
			influxdbtesting.ErrorsEqual(t, err, tt.wants.err)
		})
	}
}
