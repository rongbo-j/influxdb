// Package servicetest provides tests to ensure that implementations of
// platform/task/backend.Store and platform/task/backend.LogReader meet the requirements of influxdb.TaskService.
//
// Consumers of this package must import query/builtin.
// This package does not import it directly, to avoid requiring it too early.
package servicetest

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/influxdata/influxdb"
	icontext "github.com/influxdata/influxdb/context"
	"github.com/influxdata/influxdb/task/backend"
	"github.com/influxdata/influxdb/task/options"
)

// BackendComponentFactory is supplied by consumers of the adaptertest package,
// to provide the values required to constitute a PlatformAdapter.
// The provided context.CancelFunc is called after the test,
// and it is the implementer's responsibility to clean up after that is called.
//
// If creating the System fails, the implementer should call t.Fatal.
type BackendComponentFactory func(t *testing.T) (*System, context.CancelFunc)

// TestTaskService should be called by consumers of the servicetest package.
// This will call fn once to create a single influxdb.TaskService
// used across all subtests in TestTaskService.
func TestTaskService(t *testing.T, fn BackendComponentFactory, testCategory ...string) {
	sys, cancel := fn(t)
	defer cancel()

	if len(testCategory) == 0 {
		testCategory = []string{"transactional", "analytical"}
	}

	for _, category := range testCategory {
		switch category {
		case "transactional":
			t.Run("TransactionalTaskService", func(t *testing.T) {
				// We're running the subtests in parallel, but if we don't use this wrapper,
				// the defer cancel() call above would return before the parallel subtests completed.
				//
				// Running the subtests in parallel might make them slightly faster,
				// but more importantly, it should exercise concurrency to catch data races.

				t.Run("Task CRUD", func(t *testing.T) {
					t.Parallel()
					testTaskCRUD(t, sys)
				})

				t.Run("Task Update Options Full", func(t *testing.T) {
					t.Parallel()
					testTaskOptionsUpdateFull(t, sys)
				})

				t.Run("Task Runs", func(t *testing.T) {
					t.Parallel()
					testTaskRuns(t, sys)
				})

				t.Run("Task Concurrency", func(t *testing.T) {
					if testing.Short() {
						t.Skip("skipping in short mode")
					}
					t.Parallel()
					testTaskConcurrency(t, sys)
				})

				t.Run("Task Updates", func(t *testing.T) {
					t.Parallel()
					testUpdate(t, sys)
				})

				t.Run("Task Manual Run", func(t *testing.T) {
					t.Parallel()
					testManualRun(t, sys)
				})

				t.Run("Task Type", func(t *testing.T) {
					t.Parallel()
					testTaskType(t, sys)
				})

			})
		case "analytical":
			t.Run("AnalyticalTaskService", func(t *testing.T) {
				t.Run("Task Run Storage", func(t *testing.T) {
					t.Parallel()
					testRunStorage(t, sys)
				})
				t.Run("Task RetryRun", func(t *testing.T) {
					t.Parallel()
					testRetryAcrossStorage(t, sys)
				})
				t.Run("task Log Storage", func(t *testing.T) {
					t.Parallel()
					testLogsAcrossStorage(t, sys)
				})
			})
		}
	}

}

// TestCreds encapsulates credentials needed for a system to properly work with tasks.
type TestCreds struct {
	OrgID, UserID, AuthorizationID influxdb.ID
	Org                            string
	Token                          string
}

// Authorizer returns an authorizer for the credentials in the struct
func (tc TestCreds) Authorizer() influxdb.Authorizer {
	return &influxdb.Authorization{
		ID:     tc.AuthorizationID,
		OrgID:  tc.OrgID,
		UserID: tc.UserID,
		Token:  tc.Token,
	}
}

// UsedServices is a simple interface that contains all the service we need
// now we dont have to use a specific implementation of these services.
type UsedServices interface {
	influxdb.UserService
	influxdb.OrganizationService
	influxdb.UserResourceMappingService
	influxdb.AuthorizationService
}

// System  as in "system under test" encapsulates the required parts of a influxdb.TaskAdapter
type System struct {
	TaskControlService backend.TaskControlService

	// Used in the Creds function to create valid organizations, users, tokens, etc.
	I UsedServices

	// Set this context, to be used in tests, so that any spawned goroutines watching Ctx.Done()
	// will clean up after themselves.
	Ctx context.Context

	// TaskService is the task service we would like to test
	TaskService influxdb.TaskService

	// Override for accessing credentials for an individual test.
	// Callers can leave this nil and the test will create its own random IDs for each test.
	// However, if the system needs to verify credentials,
	// the caller should set this value and return valid IDs and a valid token.
	// It is safe if this returns the same values every time it is called.
	CredsFunc func(*testing.T) (TestCreds, error)
}

func testTaskCRUD(t *testing.T, sys *System) {
	cr := creds(t, sys)

	// Create a task.
	tc := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}

	authorizedCtx := icontext.SetAuthorizer(sys.Ctx, cr.Authorizer())

	tsk, err := sys.TaskService.CreateTask(authorizedCtx, tc)
	if err != nil {
		t.Fatal(err)
	}
	if !tsk.ID.Valid() {
		t.Fatal("no task ID set")
	}

	findTask := func(tasks []*influxdb.Task, id influxdb.ID) (*influxdb.Task, error) {
		for _, t := range tasks {
			if t.ID == id {
				return t, nil
			}
		}
		return nil, fmt.Errorf("failed to find task by id %s", id)
	}

	// TODO: replace with ErrMissingOwner test
	// // should not be able to create a task without a token
	// noToken := influxdb.TaskCreate{
	// 	OrganizationID: cr.OrgID,
	// 	Flux:           fmt.Sprintf(scriptFmt, 0),
	// 	// OwnerID:          cr.UserID, // should fail
	// }
	// _, err = sys.TaskService.CreateTask(authorizedCtx, noToken)

	// if err != influxdb.ErrMissingToken {
	// 	t.Fatalf("expected error missing token, got: %v", err)
	// }

	// Look up a task the different ways we can.
	// Map of method name to found task.
	found := map[string]*influxdb.Task{
		"Created": tsk,
	}

	// Find by ID should return the right task.
	f, err := sys.TaskService.FindTaskByID(sys.Ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	found["FindTaskByID"] = f

	fs, _, err := sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID})
	if err != nil {
		t.Fatal(err)
	}
	f, err = findTask(fs, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	found["FindTasks with Organization filter"] = f

	fs, _, err = sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{User: &cr.UserID})
	if err != nil {
		t.Fatal(err)
	}
	f, err = findTask(fs, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	found["FindTasks with User filter"] = f

	want := &influxdb.Task{
		ID:              tsk.ID,
		CreatedAt:       tsk.CreatedAt,
		LatestCompleted: tsk.LatestCompleted,
		OrganizationID:  cr.OrgID,
		Organization:    cr.Org,
		AuthorizationID: tsk.AuthorizationID,
		Authorization:   tsk.Authorization,
		OwnerID:         tsk.OwnerID,
		Name:            "task #0",
		Cron:            "* * * * *",
		Offset:          "5s",
		Status:          string(backend.DefaultTaskStatus),
		Flux:            fmt.Sprintf(scriptFmt, 0),
	}
	for fn, f := range found {
		if diff := cmp.Diff(f, want); diff != "" {
			t.Logf("got: %+#v", f)
			t.Fatalf("expected %s task to be consistant: -got/+want: %s", fn, diff)
		}
	}

	// Check limits
	tc2 := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 1),
		OwnerID:        cr.UserID,
	}

	if _, err := sys.TaskService.CreateTask(authorizedCtx, tc2); err != nil {
		t.Fatal(err)
	}
	if !tsk.ID.Valid() {
		t.Fatal("no task ID set")
	}
	tasks, _, err := sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) > 1 {
		t.Fatalf("failed to limit tasks: expected: 1, got : %d", len(tasks))
	}

	// Check after
	first := tasks[0]
	tasks, _, err = sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID, After: &first.ID})
	if err != nil {
		t.Fatal(err)
	}
	// because this test runs concurrently we can only guarantee we at least 2 tasks
	// when using after we can check to make sure the after is not in the list
	if len(tasks) == 0 {
		t.Fatalf("expected at least 1 task: got 0")
	}
	for _, task := range tasks {
		if first.ID == task.ID {
			t.Fatalf("after task included in task list")
		}
	}

	// Update task: script only.
	newFlux := fmt.Sprintf(scriptFmt, 99)
	origID := f.ID
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Flux: &newFlux})
	if err != nil {
		t.Fatal(err)
	}

	if origID != f.ID {
		t.Fatalf("task ID unexpectedly changed during update, from %s to %s", origID.String(), f.ID.String())
	}
	if f.Flux != newFlux {
		t.Fatalf("wrong flux from update; want %q, got %q", newFlux, f.Flux)
	}
	if f.Status != string(backend.TaskActive) {
		t.Fatalf("expected task to be created active, got %q", f.Status)
	}

	// Update task: status only.
	newStatus := string(backend.TaskInactive)
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Status: &newStatus})
	if err != nil {
		t.Fatal(err)
	}
	if f.Flux != newFlux {
		t.Fatalf("flux unexpected updated: %s", f.Flux)
	}
	if f.Status != newStatus {
		t.Fatalf("expected task status to be inactive, got %q", f.Status)
	}

	// Update task: reactivate status and update script.
	newStatus = string(backend.TaskActive)
	newFlux = fmt.Sprintf(scriptFmt, 98)
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Flux: &newFlux, Status: &newStatus})
	if err != nil {
		t.Fatal(err)
	}
	if f.Flux != newFlux {
		t.Fatalf("flux unexpected updated: %s", f.Flux)
	}
	if f.Status != newStatus {
		t.Fatalf("expected task status to be inactive, got %q", f.Status)
	}

	// Update task: just update an option.
	newStatus = string(backend.TaskActive)
	newFlux = "option task = {\n\tname: \"task-changed #98\",\n\tcron: \"* * * * *\",\n\toffset: 5s,\n\tconcurrency: 100,\n}\n\nfrom(bucket: \"b\")\n\t|> to(bucket: \"two\", orgID: \"000000000000000\")"
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Options: options.Options{Name: "task-changed #98"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Flux != newFlux {
		diff := cmp.Diff(f.Flux, newFlux)
		t.Fatalf("flux unexpected updated: %s", diff)
	}
	if f.Status != newStatus {
		t.Fatalf("expected task status to be active, got %q", f.Status)
	}

	// Update task: switch to every.
	newStatus = string(backend.TaskActive)
	newFlux = "option task = {\n\tname: \"task-changed #98\",\n\tevery: 30s,\n\toffset: 5s,\n\tconcurrency: 100,\n}\n\nfrom(bucket: \"b\")\n\t|> to(bucket: \"two\", orgID: \"000000000000000\")"
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Options: options.Options{Every: *(options.MustParseDuration("30s"))}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Flux != newFlux {
		diff := cmp.Diff(f.Flux, newFlux)
		t.Fatalf("flux unexpected updated: %s", diff)
	}
	if f.Status != newStatus {
		t.Fatalf("expected task status to be active, got %q", f.Status)
	}

	// Update task: just cron.
	newStatus = string(backend.TaskActive)
	newFlux = fmt.Sprintf(scriptDifferentName, 98)
	f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Options: options.Options{Cron: "* * * * *"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.Flux != newFlux {
		diff := cmp.Diff(f.Flux, newFlux)
		t.Fatalf("flux unexpected updated: %s", diff)
	}
	if f.Status != newStatus {
		t.Fatalf("expected task status to be active, got %q", f.Status)
	}

	// // Update task: use a new token on the context and modify some other option.
	// // Ensure the authorization doesn't change -- it did change at one time, which was bug https://github.com/influxdata/influxdb/issues/12218.
	// newAuthz := &influxdb.Authorization{OrgID: cr.OrgID, UserID: cr.UserID, Permissions: influxdb.OperPermissions()}
	// if err := sys.I.CreateAuthorization(sys.Ctx, newAuthz); err != nil {
	// 	t.Fatal(err)
	// }
	// newAuthorizedCtx := icontext.SetAuthorizer(sys.Ctx, newAuthz)
	// f, err = sys.TaskService.UpdateTask(newAuthorizedCtx, origID, influxdb.TaskUpdate{Options: options.Options{Name: "foo"}})
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if f.Name != "foo" {
	// 	t.Fatalf("expected name to update to foo, got %s", f.Name)
	// }
	// if f.AuthorizationID != authzID {
	// 	t.Fatalf("expected authorization ID to remain %v, got %v", authzID, f.AuthorizationID)
	// }

	// // Now actually update to use the new token, from the original authorization context.
	// f, err = sys.TaskService.UpdateTask(authorizedCtx, origID, influxdb.TaskUpdate{Token: newAuthz.Token})
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// if f.AuthorizationID != newAuthz.ID {
	// 	t.Fatalf("expected authorization ID %v, got %v", newAuthz.ID, f.AuthorizationID)
	// }

	// Delete task.
	if err := sys.TaskService.DeleteTask(sys.Ctx, origID); err != nil {
		t.Fatal(err)
	}

	// Task should not be returned.
	if _, err := sys.TaskService.FindTaskByID(sys.Ctx, origID); err != influxdb.ErrTaskNotFound {
		t.Fatalf("expected %v, got %v", influxdb.ErrTaskNotFound, err)
	}
}

//Create a new task with a Cron and Offset option
//Update the task to remove the Offset option, and change Cron to Every
//Retrieve the task again to ensure the options are now Every, without Cron or Offset
func testTaskOptionsUpdateFull(t *testing.T, sys *System) {

	script := `option task = {
	name: "task-Options-Update",
	cron: "* * * * *",
	concurrency: 100,
	offset: 10s,
}

from(bucket: "b")
	|> to(bucket: "two", orgID: "000000000000000")`

	cr := creds(t, sys)

	ct := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           script,
		OwnerID:        cr.UserID,
	}
	authorizedCtx := icontext.SetAuthorizer(sys.Ctx, cr.Authorizer())
	task, err := sys.TaskService.CreateTask(authorizedCtx, ct)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("update task and delete offset", func(t *testing.T) {
		expectedFlux := `option task = {name: "task-Options-Update", every: 10s, concurrency: 100}

from(bucket: "b")
	|> to(bucket: "two", orgID: "000000000000000")`
		f, err := sys.TaskService.UpdateTask(authorizedCtx, task.ID, influxdb.TaskUpdate{Options: options.Options{Offset: &options.Duration{}, Every: *(options.MustParseDuration("10s"))}})
		if err != nil {
			t.Fatal(err)
		}
		savedTask, err := sys.TaskService.FindTaskByID(sys.Ctx, f.ID)
		if err != nil {
			t.Fatal(err)
		}
		if savedTask.Flux != expectedFlux {
			diff := cmp.Diff(savedTask.Flux, expectedFlux)
			t.Fatalf("flux unexpected updated: %s", diff)
		}
	})
	t.Run("update task with different offset option", func(t *testing.T) {
		expectedFlux := `option task = {
	name: "task-Options-Update",
	every: 10s,
	concurrency: 100,
	offset: 10s,
}

from(bucket: "b")
	|> to(bucket: "two", orgID: "000000000000000")`
		f, err := sys.TaskService.UpdateTask(authorizedCtx, task.ID, influxdb.TaskUpdate{Options: options.Options{Offset: options.MustParseDuration("10s")}})
		if err != nil {
			t.Fatal(err)
		}
		savedTask, err := sys.TaskService.FindTaskByID(sys.Ctx, f.ID)
		if err != nil {
			t.Fatal(err)
		}
		if savedTask.Flux != expectedFlux {
			diff := cmp.Diff(savedTask.Flux, expectedFlux)
			t.Fatalf("flux unexpected updated: %s", diff)
		}
	})

}

func testUpdate(t *testing.T, sys *System) {
	cr := creds(t, sys)

	now := time.Now()
	earliestCA := now.Add(-time.Second)

	ct := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}
	authorizedCtx := icontext.SetAuthorizer(sys.Ctx, cr.Authorizer())
	task, err := sys.TaskService.CreateTask(authorizedCtx, ct)
	if err != nil {
		t.Fatal(err)
	}

	st, err := sys.TaskService.FindTaskByID(sys.Ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	after := time.Now()
	latestCA := after.Add(time.Second)

	ca, err := time.Parse(time.RFC3339, st.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}

	if earliestCA.After(ca) || latestCA.Before(ca) {
		t.Fatalf("createdAt not accurate, expected %s to be between %s and %s", ca, earliestCA, latestCA)
	}

	ti, err := time.Parse(time.RFC3339, st.LatestCompleted)
	if err != nil {
		t.Fatal(err)
	}

	if now.Sub(ti) > 10*time.Second {
		t.Fatalf("latest completed not accurate, expected: ~%s, got %s", now, ti)
	}

	requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix()

	rc, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}

	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc.Created.RunID, time.Now(), backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc.Created.RunID); err != nil {
		t.Fatal(err)
	}

	st2, err := sys.TaskService.FindTaskByID(sys.Ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	if st2.LatestCompleted <= st.LatestCompleted {
		t.Fatalf("executed task has not updated latest complete: expected %s > %s", st2.LatestCompleted, st.LatestCompleted)
	}

	now = time.Now()
	flux := fmt.Sprintf(scriptFmt, 1)
	task, err = sys.TaskService.UpdateTask(authorizedCtx, task.ID, influxdb.TaskUpdate{Flux: &flux})
	if err != nil {
		t.Fatal(err)
	}
	after = time.Now()

	earliestUA := now.Add(-time.Second)
	latestUA := after.Add(time.Second)

	ua, err := time.Parse(time.RFC3339, task.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}

	if earliestUA.After(ua) || latestUA.Before(ua) {
		t.Fatalf("updatedAt not accurate, expected %s to be between %s and %s", ua, earliestUA, latestUA)
	}

	st, err = sys.TaskService.FindTaskByID(sys.Ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}

	ua, err = time.Parse(time.RFC3339, st.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}

	if earliestUA.After(ua) || latestUA.Before(ua) {
		t.Fatalf("updatedAt not accurate after pulling new task, expected %s to be between %s and %s", ua, earliestUA, latestUA)
	}
}

func testTaskRuns(t *testing.T, sys *System) {
	cr := creds(t, sys)

	t.Run("FindRuns and FindRunByID", func(t *testing.T) {
		t.Parallel()

		// Script is set to run every minute. The platform adapter is currently hardcoded to schedule after "now",
		// which makes timing of runs somewhat difficult.
		ct := influxdb.TaskCreate{
			OrganizationID: cr.OrgID,
			Flux:           fmt.Sprintf(scriptFmt, 0),
			OwnerID:        cr.UserID,
		}
		task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
		if err != nil {
			t.Fatal(err)
		}

		// check run filter errors
		_, _, err0 := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: -1})
		if err0 != influxdb.ErrOutOfBoundsLimit {
			t.Fatalf("failed to error with out of bounds run limit: %d", -1)
		}

		_, _, err1 := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: influxdb.TaskMaxPageSize + 1})
		if err1 != influxdb.ErrOutOfBoundsLimit {
			t.Fatalf("failed to error with out of bounds run limit: %d", influxdb.TaskMaxPageSize+1)
		}

		requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix() // This should guarantee we can make two runs.

		rc0, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
		if err != nil {
			t.Fatal(err)
		}
		if rc0.Created.TaskID != task.ID {
			t.Fatalf("wrong task ID on created task: got %s, want %s", rc0.Created.TaskID, task.ID)
		}

		startedAt := time.Now().UTC()

		// Update the run state to Started; normally the scheduler would do this.
		if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc0.Created.RunID, startedAt, backend.RunStarted); err != nil {
			t.Fatal(err)
		}

		rc1, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
		if err != nil {
			t.Fatal(err)
		}
		if rc1.Created.TaskID != task.ID {
			t.Fatalf("wrong task ID on created task run: got %s, want %s", rc1.Created.TaskID, task.ID)
		}

		// Update the run state to Started; normally the scheduler would do this.
		if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt, backend.RunStarted); err != nil {
			t.Fatal(err)
		}

		runs, _, err := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}

		if len(runs) != 1 {
			t.Fatalf("expected 1 run, got %#v", runs)
		}

		// Mark the second run finished.
		if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt.Add(time.Second), backend.RunSuccess); err != nil {
			t.Fatal(err)
		}

		if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc1.Created.RunID); err != nil {
			t.Fatal(err)
		}

		// Limit 1 should only return the earlier run.
		runs, _, err = sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 1 {
			t.Fatalf("expected 1 run, got %v", runs)
		}
		if runs[0].ID != rc0.Created.RunID {
			t.Fatalf("retrieved wrong run ID; want %s, got %s", rc0.Created.RunID, runs[0].ID)
		}
		if exp := startedAt.Format(time.RFC3339Nano); runs[0].StartedAt != exp {
			t.Fatalf("unexpectedStartedAt; want %s, got %s", exp, runs[0].StartedAt)
		}
		if runs[0].Status != backend.RunStarted.String() {
			t.Fatalf("unexpected run status; want %s, got %s", backend.RunStarted.String(), runs[0].Status)
		}
		if runs[0].FinishedAt != "" {
			t.Fatalf("expected empty FinishedAt, got %q", runs[0].FinishedAt)
		}

		// Look for a run that doesn't exist.
		_, err = sys.TaskService.FindRunByID(sys.Ctx, task.ID, influxdb.ID(math.MaxUint64))
		if err == nil {
			t.Fatalf("expected %s but got %s instead", influxdb.ErrRunNotFound, err)
		}

		// look for a taskID that doesn't exist.
		_, err = sys.TaskService.FindRunByID(sys.Ctx, influxdb.ID(math.MaxUint64), runs[0].ID)
		if err == nil {
			t.Fatalf("expected %s but got %s instead", influxdb.ErrRunNotFound, err)
		}

		foundRun0, err := sys.TaskService.FindRunByID(sys.Ctx, task.ID, runs[0].ID)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(foundRun0, runs[0]); diff != "" {
			t.Fatalf("difference between listed run and found run: %s", diff)
		}
	})

	t.Run("ForceRun", func(t *testing.T) {
		t.Parallel()

		ct := influxdb.TaskCreate{
			OrganizationID: cr.OrgID,
			Flux:           fmt.Sprintf(scriptFmt, 0),
			OwnerID:        cr.UserID,
		}
		task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
		if err != nil {
			t.Fatal(err)
		}

		const scheduledFor = 77
		r, err := sys.TaskService.ForceRun(sys.Ctx, task.ID, scheduledFor)
		if err != nil {
			t.Fatal(err)
		}
		if r.ScheduledFor != "1970-01-01T00:01:17Z" {
			t.Fatalf("expected: 1970-01-01T00:01:17Z, got %s", r.ScheduledFor)
		}

		// TODO(lh): Once we have moved over to kv we can list runs and see the manual queue in the list

		// Forcing the same run before it's executed should be rejected.
		if _, err = sys.TaskService.ForceRun(sys.Ctx, task.ID, scheduledFor); err == nil {
			t.Fatalf("subsequent force should have been rejected; failed to error: %s", task.ID)
		}
	})

	t.Run("FindLogs", func(t *testing.T) {
		t.Parallel()

		ct := influxdb.TaskCreate{
			OrganizationID: cr.OrgID,
			Flux:           fmt.Sprintf(scriptFmt, 0),
			OwnerID:        cr.UserID,
		}
		task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
		if err != nil {
			t.Fatal(err)
		}

		requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix() // This should guarantee we can make a run.

		// Create two runs.
		rc1, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
		if err != nil {
			t.Fatal(err)
		}
		if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, time.Now(), backend.RunStarted); err != nil {
			t.Fatal(err)
		}

		rc2, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
		if err != nil {
			t.Fatal(err)
		}
		if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc2.Created.RunID, time.Now(), backend.RunStarted); err != nil {
			t.Fatal(err)
		}
		// Add a log for the first run.
		log1Time := time.Now().UTC()
		if err := sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc1.Created.RunID, log1Time, "entry 1"); err != nil {
			t.Fatal(err)
		}

		// Ensure it is returned when filtering logs by run ID.
		logs, _, err := sys.TaskService.FindLogs(sys.Ctx, influxdb.LogFilter{
			Task: task.ID,
			Run:  &rc1.Created.RunID,
		})
		if err != nil {
			t.Fatal(err)
		}

		expLine1 := &influxdb.Log{RunID: rc1.Created.RunID, Time: log1Time.Format(time.RFC3339Nano), Message: "entry 1"}
		exp := []*influxdb.Log{expLine1}
		if diff := cmp.Diff(logs, exp); diff != "" {
			t.Fatalf("unexpected log: -got/+want: %s", diff)
		}

		// Add a log for the second run.
		log2Time := time.Now().UTC()
		if err := sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc2.Created.RunID, log2Time, "entry 2"); err != nil {
			t.Fatal(err)
		}

		// Ensure both returned when filtering logs by task ID.
		logs, _, err = sys.TaskService.FindLogs(sys.Ctx, influxdb.LogFilter{
			Task: task.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		expLine2 := &influxdb.Log{RunID: rc2.Created.RunID, Time: log2Time.Format(time.RFC3339Nano), Message: "entry 2"}
		exp = []*influxdb.Log{expLine1, expLine2}
		if diff := cmp.Diff(logs, exp); diff != "" {
			t.Fatalf("unexpected log: -got/+want: %s", diff)
		}
	})
}

func testTaskConcurrency(t *testing.T, sys *System) {
	cr := creds(t, sys)

	const numTasks = 450 // Arbitrarily chosen to get a reasonable count of concurrent creates and deletes.
	createTaskCh := make(chan influxdb.TaskCreate, numTasks)

	// Since this test is run in parallel with other tests,
	// we need to keep a whitelist of IDs that are okay to delete.
	// This only matters when the creds function returns an identical user/org from another test.
	var idMu sync.Mutex
	taskIDs := make(map[influxdb.ID]struct{})

	var createWg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		createWg.Add(1)
		go func() {
			defer createWg.Done()
			aCtx := icontext.SetAuthorizer(sys.Ctx, cr.Authorizer())
			for ct := range createTaskCh {
				task, err := sys.TaskService.CreateTask(aCtx, ct)
				if err != nil {
					t.Errorf("error creating task: %v", err)
					continue
				}
				idMu.Lock()
				taskIDs[task.ID] = struct{}{}
				idMu.Unlock()
			}
		}()
	}

	// Signal for non-creator goroutines to stop.
	quitCh := make(chan struct{})
	go func() {
		createWg.Wait()
		close(quitCh)
	}()

	var extraWg sync.WaitGroup
	// Get all the tasks, and delete the first one we find.
	extraWg.Add(1)
	go func() {
		defer extraWg.Done()

		deleted := 0
		defer func() {
			t.Logf("Concurrently deleted %d tasks", deleted)
		}()
		for {
			// Check if we need to quit.
			select {
			case <-quitCh:
				return
			default:
			}

			// Get all the tasks.
			tasks, _, err := sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID})
			if err != nil {
				t.Errorf("error finding tasks: %v", err)
				return
			}
			if len(tasks) == 0 {
				continue
			}

			// Check again if we need to quit.
			select {
			case <-quitCh:
				return
			default:
			}

			for _, tsk := range tasks {
				// Was the retrieved task an ID we're allowed to delete?
				idMu.Lock()
				_, ok := taskIDs[tsk.ID]
				idMu.Unlock()
				if !ok {
					continue
				}

				// Task was in whitelist. Delete it from the TaskService.
				// We could remove it from the taskIDs map, but this test is short-lived enough
				// that clearing out the map isn't really worth taking the lock again.
				if err := sys.TaskService.DeleteTask(sys.Ctx, tsk.ID); err != nil {
					t.Errorf("error deleting task: %v", err)
					return
				}
				deleted++

				// Wait just a tiny bit.
				time.Sleep(time.Millisecond)
				break
			}
		}
	}()

	extraWg.Add(1)
	go func() {
		defer extraWg.Done()

		runsCreated := 0
		defer func() {
			t.Logf("Concurrently created %d runs", runsCreated)
		}()
		for {
			// Check if we need to quit.
			select {
			case <-quitCh:
				return
			default:
			}

			// Get all the tasks.
			tasks, _, err := sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID})
			if err != nil {
				t.Errorf("error finding tasks: %v", err)
				return
			}
			if len(tasks) == 0 {
				continue
			}

			// Check again if we need to quit.
			select {
			case <-quitCh:
				return
			default:
			}

			// Create a run for the last task we found.
			// The script should run every minute, so use max now.
			var tid influxdb.ID
			idMu.Lock()
			for i := len(tasks) - 1; i >= 0; i-- {
				_, ok := taskIDs[tasks[i].ID]
				if ok {
					tid = tasks[i].ID
					break
				}
			}
			idMu.Unlock()
			if !tid.Valid() {
				continue
			}
			if _, err := sys.TaskControlService.CreateNextRun(sys.Ctx, tid, math.MaxInt64>>6); err != nil { // we use the >>6 here because math.MaxInt64 is too large which causes problems when converting back and forth from time
				// This may have errored due to the task being deleted. Check if the task still exists.

				if _, err2 := sys.TaskService.FindTaskByID(sys.Ctx, tid); err2 == influxdb.ErrTaskNotFound {
					// It was deleted. Just continue.
					continue
				}
				// Otherwise, we were able to find the task, so something went wrong here.
				t.Errorf("error creating next run: %v", err)
				return
			}
			runsCreated++

			// Wait just a tiny bit.
			time.Sleep(time.Millisecond)
		}
	}()

	// Start adding tasks.
	for i := 0; i < numTasks; i++ {
		createTaskCh <- influxdb.TaskCreate{
			OrganizationID: cr.OrgID,
			Flux:           fmt.Sprintf(scriptFmt, i),
			OwnerID:        cr.UserID,
		}
	}

	// Done adding. Wait for cleanup.
	close(createTaskCh)
	createWg.Wait()
	extraWg.Wait()
}

func testManualRun(t *testing.T, s *System) {
	cr := creds(t, s)

	// Create a task.
	tc := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}

	authorizedCtx := icontext.SetAuthorizer(s.Ctx, cr.Authorizer())

	tsk, err := s.TaskService.CreateTask(authorizedCtx, tc)
	if err != nil {
		t.Fatal(err)
	}
	if !tsk.ID.Valid() {
		t.Fatal("no task ID set")
	}
	scheduledFor := time.Now().UTC()

	run, err := s.TaskService.ForceRun(authorizedCtx, tsk.ID, scheduledFor.Unix())
	if err != nil {
		t.Fatal(err)
	}

	if run.ScheduledFor != scheduledFor.Format(time.RFC3339) {
		t.Fatalf("force run returned a different scheduled for time expected: %s, got %s", scheduledFor.Format(time.RFC3339), run.ScheduledFor)
	}

	runs, err := s.TaskControlService.ManualRuns(authorizedCtx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 manual run: got %d", len(runs))
	}
	if runs[0].ID != run.ID {
		diff := cmp.Diff(runs[0], run)
		t.Fatalf("manual run missmatch: %s", diff)
	}
}

func testRunStorage(t *testing.T, sys *System) {
	cr := creds(t, sys)

	// Script is set to run every minute. The platform adapter is currently hardcoded to schedule after "now",
	// which makes timing of runs somewhat difficult.
	ct := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}
	task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
	if err != nil {
		t.Fatal(err)
	}

	// check run filter errors
	_, _, err0 := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: -1})
	if err0 != influxdb.ErrOutOfBoundsLimit {
		t.Fatalf("failed to error with out of bounds run limit: %d", -1)
	}

	_, _, err1 := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: influxdb.TaskMaxPageSize + 1})
	if err1 != influxdb.ErrOutOfBoundsLimit {
		t.Fatalf("failed to error with out of bounds run limit: %d", influxdb.TaskMaxPageSize+1)
	}

	requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix() // This should guarantee we can make two runs.

	rc0, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if rc0.Created.TaskID != task.ID {
		t.Fatalf("wrong task ID on created task: got %s, want %s", rc0.Created.TaskID, task.ID)
	}

	startedAt := time.Now().UTC().Add(time.Second * -10)

	// Update the run state to Started; normally the scheduler would do this.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc0.Created.RunID, startedAt, backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	rc1, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if rc1.Created.TaskID != task.ID {
		t.Fatalf("wrong task ID on created task run: got %s, want %s", rc1.Created.TaskID, task.ID)
	}

	// Update the run state to Started; normally the scheduler would do this.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt.Add(time.Second), backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	// Mark the second run finished.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt.Add(time.Second*2), backend.RunFail); err != nil {
		t.Fatal(err)
	}

	if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc1.Created.RunID); err != nil {
		t.Fatal(err)
	}

	// Limit 1 should only return the earlier run.
	runs, _, err := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %v", runs)
	}
	if runs[0].ID != rc0.Created.RunID {
		t.Fatalf("retrieved wrong run ID; want %s, got %s", rc0.Created.RunID, runs[0].ID)
	}
	if exp := startedAt.Format(time.RFC3339Nano); runs[0].StartedAt != exp {
		t.Fatalf("unexpectedStartedAt; want %s, got %s", exp, runs[0].StartedAt)
	}
	if runs[0].Status != backend.RunStarted.String() {
		t.Fatalf("unexpected run status; want %s, got %s", backend.RunStarted.String(), runs[0].Status)
	}
	if runs[0].FinishedAt != "" {
		t.Fatalf("expected empty FinishedAt, got %q", runs[0].FinishedAt)
	}

	// Create 3rd run and test limiting to 2 runs
	rc2, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc2.Created.RunID, startedAt.Add(time.Second*3), backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc2.Created.RunID, startedAt.Add(time.Second*4), backend.RunSuccess); err != nil {
		t.Fatal(err)
	}
	if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc2.Created.RunID); err != nil {
		t.Fatal(err)
	}

	runs2, _, err := sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs2) != 2 {
		t.Fatalf("expected 2 runs, got %v", runs2)
	}
	if runs2[0].ID != rc0.Created.RunID {
		t.Fatalf("retrieved wrong run ID; want %s, got %s", rc0.Created.RunID, runs[0].ID)
	}

	// Unspecified limit returns all three runs, sorted by most recently scheduled first.
	runs, _, err = sys.TaskService.FindRuns(sys.Ctx, influxdb.RunFilter{Task: task.ID})

	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %v", runs)
	}
	if runs[0].ID != rc0.Created.RunID {
		t.Fatalf("retrieved wrong run ID; want %s, got %s", rc0.Created.RunID, runs[0].ID)
	}
	if exp := startedAt.Format(time.RFC3339Nano); runs[0].StartedAt != exp {
		t.Fatalf("unexpectedStartedAt; want %s, got %s", exp, runs[0].StartedAt)
	}
	if runs[0].Status != backend.RunStarted.String() {
		t.Fatalf("unexpected run status; want %s, got %s", backend.RunStarted.String(), runs[0].Status)
	}
	if runs[0].FinishedAt != "" {
		t.Fatalf("expected empty FinishedAt, got %q", runs[0].FinishedAt)
	}

	if runs[2].ID != rc1.Created.RunID {
		t.Fatalf("retrieved wrong run ID; want %s, got %s", rc2.Created.RunID, runs[1].ID)
	}
	if runs[2].StartedAt != startedAt.Add(time.Second).Format(time.RFC3339Nano) {
		t.Fatalf("unexpected StartedAt; want %s, got %s", runs[1].StartedAt, startedAt.Add(time.Second))
	}
	if runs[2].Status != backend.RunFail.String() {
		t.Fatalf("unexpected run status; want %s, got %s", backend.RunSuccess.String(), runs[2].Status)
	}
	if exp := startedAt.Add(time.Second * 2).Format(time.RFC3339Nano); runs[2].FinishedAt != exp {
		t.Fatalf("unexpected FinishedAt; want %s, got %s", exp, runs[2].FinishedAt)
	}

	// Look for a run that doesn't exist.
	_, err = sys.TaskService.FindRunByID(sys.Ctx, task.ID, influxdb.ID(math.MaxUint64))
	// TODO(lh): use kv.ErrRunNotFound in the future. Our error's are not exact
	if err == nil {
		t.Fatalf("expected %s but got %s instead", influxdb.ErrRunNotFound, err)
	}

	// look for a taskID that doesn't exist.
	_, err = sys.TaskService.FindRunByID(sys.Ctx, influxdb.ID(math.MaxUint64), runs[0].ID)
	if err == nil {
		t.Fatalf("expected %s but got %s instead", influxdb.ErrRunNotFound, err)
	}

	foundRun0, err := sys.TaskService.FindRunByID(sys.Ctx, task.ID, runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(foundRun0, runs[0]); diff != "" {
		t.Fatalf("difference between listed run and found run: %s", diff)
	}

	foundRun1, err := sys.TaskService.FindRunByID(sys.Ctx, task.ID, runs[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(foundRun1, runs[1]); diff != "" {
		t.Fatalf("difference between listed run and found run: %s", diff)
	}
}

func testRetryAcrossStorage(t *testing.T, sys *System) {
	cr := creds(t, sys)

	// Script is set to run every minute.
	ct := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}
	task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
	if err != nil {
		t.Fatal(err)
	}
	// Non-existent ID should return the right error.
	_, err = sys.TaskService.RetryRun(sys.Ctx, task.ID, influxdb.ID(math.MaxUint64))
	if !strings.Contains(err.Error(), "run not found") {
		t.Errorf("expected retrying run that doesn't exist to return %v, got %v", influxdb.ErrRunNotFound, err)
	}

	requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix() // This should guarantee we can make a run.

	rc, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Created.TaskID != task.ID {
		t.Fatalf("wrong task ID on created task: got %s, want %s", rc.Created.TaskID, task.ID)
	}

	startedAt := time.Now().UTC()

	// Update the run state to Started then Failed; normally the scheduler would do this.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc.Created.RunID, startedAt, backend.RunStarted); err != nil {
		t.Fatal(err)
	}
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc.Created.RunID, startedAt.Add(time.Second), backend.RunFail); err != nil {
		t.Fatal(err)
	}
	if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc.Created.RunID); err != nil {
		t.Fatal(err)
	}

	// Now retry the run.
	m, err := sys.TaskService.RetryRun(sys.Ctx, task.ID, rc.Created.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.TaskID != task.ID {
		t.Fatalf("wrong task ID on retried run: got %s, want %s", m.TaskID, task.ID)
	}
	if m.Status != "scheduled" {
		t.Fatal("expected new retried run to have status of scheduled")
	}
	nowTime, err := time.Parse(time.RFC3339, m.ScheduledFor)
	if err != nil {
		t.Fatalf("expected scheduledFor to be a parsable time in RFC3339, but got %s", m.ScheduledFor)
	}
	if nowTime.Unix() != rc.Created.Now {
		t.Fatalf("wrong scheduledFor on task: got %s, want %s", m.ScheduledFor, time.Unix(rc.Created.Now, 0).Format(time.RFC3339))
	}

	exp := backend.RequestStillQueuedError{Start: rc.Created.Now, End: rc.Created.Now}

	// Retrying a run which has been queued but not started, should be rejected.
	if _, err = sys.TaskService.RetryRun(sys.Ctx, task.ID, rc.Created.RunID); err != exp && err.Error() != "<conflict> run already queued" {
		t.Fatalf("subsequent retry should have been rejected with %v; got %v", exp, err)
	}
}

func testLogsAcrossStorage(t *testing.T, sys *System) {
	cr := creds(t, sys)

	// Script is set to run every minute. The platform adapter is currently hardcoded to schedule after "now",
	// which makes timing of runs somewhat difficult.
	ct := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}
	task, err := sys.TaskService.CreateTask(icontext.SetAuthorizer(sys.Ctx, cr.Authorizer()), ct)
	if err != nil {
		t.Fatal(err)
	}

	requestedAtUnix := time.Now().Add(5 * time.Minute).UTC().Unix() // This should guarantee we can make two runs.

	rc0, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if rc0.Created.TaskID != task.ID {
		t.Fatalf("wrong task ID on created task: got %s, want %s", rc0.Created.TaskID, task.ID)
	}

	startedAt := time.Now().UTC()

	// Update the run state to Started; normally the scheduler would do this.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc0.Created.RunID, startedAt, backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	rc1, err := sys.TaskControlService.CreateNextRun(sys.Ctx, task.ID, requestedAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if rc1.Created.TaskID != task.ID {
		t.Fatalf("wrong task ID on created task run: got %s, want %s", rc1.Created.TaskID, task.ID)
	}

	// Update the run state to Started; normally the scheduler would do this.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt, backend.RunStarted); err != nil {
		t.Fatal(err)
	}

	// Mark the second run finished.
	if err := sys.TaskControlService.UpdateRunState(sys.Ctx, task.ID, rc1.Created.RunID, startedAt.Add(time.Second), backend.RunSuccess); err != nil {
		t.Fatal(err)
	}

	// Create several run logs in both rc0 and rc1
	// We can then finalize rc1 and ensure that both the transactional (currently running logs) can be found with analytical (completed) logs.
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc0.Created.RunID, time.Now(), "0-0")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc0.Created.RunID, time.Now(), "0-1")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc0.Created.RunID, time.Now(), "0-2")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc1.Created.RunID, time.Now(), "1-0")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc1.Created.RunID, time.Now(), "1-1")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc1.Created.RunID, time.Now(), "1-2")
	sys.TaskControlService.AddRunLog(sys.Ctx, task.ID, rc1.Created.RunID, time.Now(), "1-3")
	if _, err := sys.TaskControlService.FinishRun(sys.Ctx, task.ID, rc1.Created.RunID); err != nil {
		t.Fatal(err)
	}

	logs, _, err := sys.TaskService.FindLogs(sys.Ctx, influxdb.LogFilter{Task: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 7 {
		for _, log := range logs {
			t.Logf("log: %+v\n", log)
		}
		t.Fatalf("failed to get all logs: expected: 7 got: %d", len(logs))
	}
	smash := func(logs []*influxdb.Log) string {
		smashed := ""
		for _, log := range logs {
			smashed = smashed + log.Message
		}
		return smashed
	}
	if smash(logs) != "0-00-10-21-01-11-21-3" {
		t.Fatalf("log contents not acceptable, expected: %q, got: %q", "0-00-10-21-01-11-21-3", smash(logs))
	}

	logs, _, err = sys.TaskService.FindLogs(sys.Ctx, influxdb.LogFilter{Task: task.ID, Run: &rc1.Created.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 4 {
		t.Fatalf("failed to get all logs: expected: 4 got: %d", len(logs))
	}

	if smash(logs) != "1-01-11-21-3" {
		t.Fatalf("log contents not acceptable, expected: %q, got: %q", "1-01-11-21-3", smash(logs))
	}

	logs, _, err = sys.TaskService.FindLogs(sys.Ctx, influxdb.LogFilter{Task: task.ID, Run: &rc0.Created.RunID})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 {
		t.Fatalf("failed to get all logs: expected: 3 got: %d", len(logs))
	}

	if smash(logs) != "0-00-10-2" {
		t.Fatalf("log contents not acceptable, expected: %q, got: %q", "0-00-10-2", smash(logs))
	}

}

func creds(t *testing.T, s *System) TestCreds {
	t.Helper()

	if s.CredsFunc == nil {
		u := &influxdb.User{Name: t.Name() + "-user"}
		if err := s.I.CreateUser(s.Ctx, u); err != nil {
			t.Fatal(err)
		}
		o := &influxdb.Organization{Name: t.Name() + "-org"}
		if err := s.I.CreateOrganization(s.Ctx, o); err != nil {
			t.Fatal(err)
		}

		if err := s.I.CreateUserResourceMapping(s.Ctx, &influxdb.UserResourceMapping{
			ResourceType: influxdb.OrgsResourceType,
			ResourceID:   o.ID,
			UserID:       u.ID,
			UserType:     influxdb.Owner,
		}); err != nil {
			t.Fatal(err)
		}

		authz := influxdb.Authorization{
			OrgID:       o.ID,
			UserID:      u.ID,
			Permissions: influxdb.OperPermissions(),
		}
		if err := s.I.CreateAuthorization(context.Background(), &authz); err != nil {
			t.Fatal(err)
		}
		return TestCreds{
			OrgID:           o.ID,
			Org:             o.Name,
			UserID:          u.ID,
			AuthorizationID: authz.ID,
			Token:           authz.Token,
		}
	}

	c, err := s.CredsFunc(t)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const (
	scriptFmt = `option task = {
	name: "task #%d",
	cron: "* * * * *",
	offset: 5s,
	concurrency: 100,
}

from(bucket:"b")
	|> to(bucket: "two", orgID: "000000000000000")`

	scriptDifferentName = `option task = {
	name: "task-changed #%d",
	cron: "* * * * *",
	offset: 5s,
	concurrency: 100,
}

from(bucket: "b")
	|> to(bucket: "two", orgID: "000000000000000")`
)

func testTaskType(t *testing.T, sys *System) {
	cr := creds(t, sys)
	authorizedCtx := icontext.SetAuthorizer(sys.Ctx, cr.Authorizer())

	// Create a tasks
	ts := influxdb.TaskCreate{
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}

	tsk, err := sys.TaskService.CreateTask(authorizedCtx, ts)
	if err != nil {
		t.Fatal(err)
	}
	if !tsk.ID.Valid() {
		t.Fatal("no task ID set")
	}

	tc := influxdb.TaskCreate{
		Type:           "cows",
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}

	tskCow, err := sys.TaskService.CreateTask(authorizedCtx, tc)
	if err != nil {
		t.Fatal(err)
	}
	if !tskCow.ID.Valid() {
		t.Fatal("no task ID set")
	}

	tp := influxdb.TaskCreate{
		Type:           "pigs",
		OrganizationID: cr.OrgID,
		Flux:           fmt.Sprintf(scriptFmt, 0),
		OwnerID:        cr.UserID,
	}

	tskPig, err := sys.TaskService.CreateTask(authorizedCtx, tp)
	if err != nil {
		t.Fatal(err)
	}
	if !tskPig.ID.Valid() {
		t.Fatal("no task ID set")
	}

	// get default tasks
	tasks, _, err := sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID})
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range tasks {
		if task.Type != "" {
			t.Fatal("recieved a task with a type when sending no type restriction")
		}
	}

	// get filtered tasks
	tasks, _, err = sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID, Type: &tc.Type})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatalf("failed to return tasks by type, expected 1, got %d", len(tasks))
	}

	// get all tasks
	wc := influxdb.TaskTypeWildcard
	tasks, _, err = sys.TaskService.FindTasks(sys.Ctx, influxdb.TaskFilter{OrganizationID: &cr.OrgID, Type: &wc})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 3 {
		t.Fatalf("failed to return tasks with wildcard, expected 3, got %d", len(tasks))
	}
}
