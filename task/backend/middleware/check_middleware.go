package middleware

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb"
)

// CoordinatingCheckService acts as a CheckService decorator that handles coordinating the api request
// with the required task control actions asynchronously via a message dispatcher
type CoordinatingCheckService struct {
	influxdb.CheckService
	coordinator Coordinator
	taskService influxdb.TaskService
}

// NewCheckService constructs a new coordinating check service
func NewCheckService(cs influxdb.CheckService, ts influxdb.TaskService, coordinator Coordinator) *CoordinatingCheckService {
	c := &CoordinatingCheckService{
		CheckService: cs,
		taskService:  ts,
		coordinator:  coordinator,
	}

	return c
}

// CreateCheck Creates a check and Publishes the change it can be scheduled.
func (cs *CoordinatingCheckService) CreateCheck(ctx context.Context, c influxdb.Check, userID influxdb.ID) error {

	if err := cs.CheckService.CreateCheck(ctx, c, userID); err != nil {
		return err
	}

	t, err := cs.taskService.FindTaskByID(ctx, c.GetTaskID())
	if err != nil {
		return err
	}

	if err := cs.coordinator.TaskCreated(ctx, t); err != nil {
		if derr := cs.CheckService.DeleteCheck(ctx, c.GetID()); derr != nil {
			return fmt.Errorf("schedule task failed: %s\n\tcleanup also failed: %s", err, derr)
		}

		return err
	}

	return nil
}

// UpdateCheck Updates a check and publishes the change so the task owner can act on the update
func (cs *CoordinatingCheckService) UpdateCheck(ctx context.Context, id influxdb.ID, c influxdb.Check) (influxdb.Check, error) {
	from, err := cs.CheckService.FindCheckByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fromTask, err := cs.taskService.FindTaskByID(ctx, from.GetTaskID())
	if err != nil {
		return nil, err
	}

	to, err := cs.CheckService.UpdateCheck(ctx, id, c)
	if err != nil {
		return to, err
	}

	toTask, err := cs.taskService.FindTaskByID(ctx, to.GetTaskID())
	if err != nil {
		return nil, err
	}

	return to, cs.coordinator.TaskUpdated(ctx, fromTask, toTask)
}

// PatchCheck Updates a check and publishes the change so the task owner can act on the update
func (cs *CoordinatingCheckService) PatchCheck(ctx context.Context, id influxdb.ID, upd influxdb.CheckUpdate) (influxdb.Check, error) {
	from, err := cs.CheckService.FindCheckByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fromTask, err := cs.taskService.FindTaskByID(ctx, from.GetTaskID())
	if err != nil {
		return nil, err
	}

	to, err := cs.CheckService.PatchCheck(ctx, id, upd)
	if err != nil {
		return to, err
	}

	toTask, err := cs.taskService.FindTaskByID(ctx, to.GetTaskID())
	if err != nil {
		return nil, err
	}

	return to, cs.coordinator.TaskUpdated(ctx, fromTask, toTask)

}

// DeleteCheck delete the check and publishes the change, to allow the task owner to find out about this change faster.
func (cs *CoordinatingCheckService) DeleteCheck(ctx context.Context, id influxdb.ID) error {
	check, err := cs.CheckService.FindCheckByID(ctx, id)
	if err != nil {
		return err
	}

	t, err := cs.taskService.FindTaskByID(ctx, check.GetTaskID())
	if err != nil {
		return err
	}

	if err := cs.coordinator.TaskDeleted(ctx, t.ID); err != nil {
		return err
	}

	return cs.CheckService.DeleteCheck(ctx, id)
}
