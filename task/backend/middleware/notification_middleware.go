package middleware

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb"
)

// CoordinatingNotificationRuleStore acts as a NotificationRuleStore decorator that handles coordinating the api request
// with the required task control actions asynchronously via a message dispatcher
type CoordinatingNotificationRuleStore struct {
	influxdb.NotificationRuleStore
	coordinator Coordinator
	taskService influxdb.TaskService
}

// NewNotificationRuleStore constructs a new coordinating notification service
func NewNotificationRuleStore(ns influxdb.NotificationRuleStore, ts influxdb.TaskService, coordinator Coordinator) *CoordinatingNotificationRuleStore {
	c := &CoordinatingNotificationRuleStore{
		NotificationRuleStore: ns,
		taskService:           ts,
		coordinator:           coordinator,
	}

	return c
}

// CreateNotificationRule Creates a notification and Publishes the change it can be scheduled.
func (ns *CoordinatingNotificationRuleStore) CreateNotificationRule(ctx context.Context, nr influxdb.NotificationRule, userID influxdb.ID) error {

	if err := ns.NotificationRuleStore.CreateNotificationRule(ctx, nr, userID); err != nil {
		return err
	}

	t, err := ns.taskService.FindTaskByID(ctx, nr.GetTaskID())
	if err != nil {
		return err
	}

	if err := ns.coordinator.TaskCreated(ctx, t); err != nil {
		if derr := ns.NotificationRuleStore.DeleteNotificationRule(ctx, nr.GetID()); derr != nil {
			return fmt.Errorf("schedule task failed: %s\n\tcleanup also failed: %s", err, derr)
		}

		return err
	}

	return nil
}

// UpdateNotificationRule Updates a notification and publishes the change so the task owner can act on the update
func (ns *CoordinatingNotificationRuleStore) UpdateNotificationRule(ctx context.Context, id influxdb.ID, nr influxdb.NotificationRule, uid influxdb.ID) (influxdb.NotificationRule, error) {
	from, err := ns.NotificationRuleStore.FindNotificationRuleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fromTask, err := ns.taskService.FindTaskByID(ctx, from.GetTaskID())
	if err != nil {
		return nil, err
	}

	to, err := ns.NotificationRuleStore.UpdateNotificationRule(ctx, id, nr, uid)
	if err != nil {
		return to, err
	}

	toTask, err := ns.taskService.FindTaskByID(ctx, to.GetTaskID())
	if err != nil {
		return nil, err
	}

	return to, ns.coordinator.TaskUpdated(ctx, fromTask, toTask)
}

// PatchNotificationRule Updates a notification and publishes the change so the task owner can act on the update
func (ns *CoordinatingNotificationRuleStore) PatchNotificationRule(ctx context.Context, id influxdb.ID, upd influxdb.NotificationRuleUpdate) (influxdb.NotificationRule, error) {
	from, err := ns.NotificationRuleStore.FindNotificationRuleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	fromTask, err := ns.taskService.FindTaskByID(ctx, from.GetTaskID())
	if err != nil {
		return nil, err
	}

	to, err := ns.NotificationRuleStore.PatchNotificationRule(ctx, id, upd)
	if err != nil {
		return to, err
	}

	toTask, err := ns.taskService.FindTaskByID(ctx, to.GetTaskID())
	if err != nil {
		return nil, err
	}

	return to, ns.coordinator.TaskUpdated(ctx, fromTask, toTask)

}

// DeleteNotificationRule delete the notification and publishes the change, to allow the task owner to find out about this change faster.
func (ns *CoordinatingNotificationRuleStore) DeleteNotificationRule(ctx context.Context, id influxdb.ID) error {
	notification, err := ns.NotificationRuleStore.FindNotificationRuleByID(ctx, id)
	if err != nil {
		return err
	}

	t, err := ns.taskService.FindTaskByID(ctx, notification.GetTaskID())
	if err != nil {
		return err
	}

	if err := ns.coordinator.TaskDeleted(ctx, t.ID); err != nil {
		return err
	}

	return ns.NotificationRuleStore.DeleteNotificationRule(ctx, id)
}
