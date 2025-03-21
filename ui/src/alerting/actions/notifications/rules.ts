// Libraries
import {Dispatch} from 'react'

// Constants
import * as copy from 'src/shared/copy/notifications'

// APIs
import * as api from 'src/client'

// Actions
import {
  notify,
  Action as NotificationAction,
} from 'src/shared/actions/notifications'

// Utils
import {
  ruleToDraftRule,
  draftRuleToNewRule,
  draftRuleToRule,
} from 'src/alerting/components/notifications/utils'

// Types
import {RemoteDataState} from '@influxdata/clockface'
import {
  NotificationRuleUpdate,
  NotificationRule,
  GetState,
  NotificationRuleDraft,
  Label,
} from 'src/types'
import {incrementCloneName} from 'src/utils/naming'

export type Action =
  | ReturnType<typeof setAllNotificationRules>
  | ReturnType<typeof setRule>
  | ReturnType<typeof setCurrentRule>
  | ReturnType<typeof removeRule>
  | {
      type: 'ADD_LABEL_TO_RULE'
      ruleID: string
      label: Label
    }
  | {
      type: 'REMOVE_LABEL_FROM_RULE'
      ruleID: string
      label: Label
    }

export const setAllNotificationRules = (
  status: RemoteDataState,
  rules?: NotificationRuleDraft[]
) => ({
  type: 'SET_ALL_NOTIFICATION_RULES' as 'SET_ALL_NOTIFICATION_RULES',
  payload: {status, rules},
})

export const setRule = (rule: NotificationRuleDraft) => ({
  type: 'SET_NOTIFICATION_RULE' as 'SET_NOTIFICATION_RULE',
  payload: {rule},
})

export const setCurrentRule = (
  status: RemoteDataState,
  rule?: NotificationRuleDraft
) => ({
  type: 'SET_CURRENT_NOTIFICATION_RULE' as 'SET_CURRENT_NOTIFICATION_RULE',
  payload: {status, rule},
})

export const removeRule = (ruleID: string) => ({
  type: 'REMOVE_NOTIFICATION_RULE' as 'REMOVE_NOTIFICATION_RULE',
  payload: {ruleID},
})

export const getNotificationRules = () => async (
  dispatch: Dispatch<Action | NotificationAction>,
  getState: GetState
) => {
  try {
    dispatch(setAllNotificationRules(RemoteDataState.Loading))
    const {
      orgs: {
        org: {id: orgID},
      },
    } = getState()

    const resp = await api.getNotificationRules({query: {orgID}})

    if (resp.status !== 200) {
      throw new Error(resp.data.message)
    }

    const draftRules = resp.data.notificationRules.map(ruleToDraftRule)

    dispatch(setAllNotificationRules(RemoteDataState.Done, draftRules))
  } catch (e) {
    console.error(e)
    dispatch(setAllNotificationRules(RemoteDataState.Error))
    dispatch(notify(copy.getNotificationRulesFailed(e.message)))
  }
}

export const getCurrentRule = (ruleID: string) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  try {
    dispatch(setCurrentRule(RemoteDataState.Loading))

    const resp = await api.getNotificationRule({ruleID})

    if (resp.status !== 200) {
      throw new Error(resp.data.message)
    }

    dispatch(setCurrentRule(RemoteDataState.Done, ruleToDraftRule(resp.data)))
  } catch (e) {
    console.error(e)
    dispatch(setCurrentRule(RemoteDataState.Error))
    dispatch(notify(copy.getNotificationRuleFailed(e.message)))
  }
}

export const createRule = (rule: NotificationRuleDraft) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  const data = draftRuleToNewRule(rule) as NotificationRule

  const resp = await api.postNotificationRule({data})

  if (resp.status !== 201) {
    throw new Error(resp.data.message)
  }

  dispatch(setRule(ruleToDraftRule(resp.data)))
}

export const updateRule = (rule: NotificationRuleDraft) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  const resp = await api.putNotificationRule({
    ruleID: rule.id,
    data: draftRuleToRule(rule),
  })

  if (resp.status !== 200) {
    throw new Error(resp.data.message)
  }

  dispatch(setRule(ruleToDraftRule(resp.data)))
}

export const updateRuleProperties = (
  ruleID: string,
  properties: NotificationRuleUpdate
) => async (dispatch: Dispatch<Action | NotificationAction>) => {
  const resp = await api.patchNotificationRule({
    ruleID,
    data: properties,
  })

  if (resp.status !== 200) {
    throw new Error(resp.data.message)
  }

  dispatch(setRule(ruleToDraftRule(resp.data)))
}

export const deleteRule = (ruleID: string) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  const resp = await api.deleteNotificationRule({ruleID})

  if (resp.status !== 204) {
    throw new Error(resp.data.message)
  }

  dispatch(removeRule(ruleID))
}

export const addRuleLabel = (ruleID: string, label: Label) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  try {
    const resp = await api.postNotificationRulesLabel({
      ruleID,
      data: {labelID: label.id},
    })

    if (resp.status !== 201) {
      throw new Error(resp.data.message)
    }

    dispatch({type: 'ADD_LABEL_TO_RULE', ruleID, label})
  } catch (e) {
    console.error(e)
  }
}

export const deleteRuleLabel = (ruleID: string, label: Label) => async (
  dispatch: Dispatch<Action | NotificationAction>
) => {
  try {
    const resp = await api.deleteNotificationRulesLabel({
      ruleID,
      labelID: label.id,
    })

    if (resp.status !== 204) {
      throw new Error(resp.data.message)
    }

    dispatch({type: 'REMOVE_LABEL_FROM_RULE', ruleID, label})
  } catch (e) {
    console.error(e)
  }
}

export const cloneRule = (draftRule: NotificationRuleDraft) => async (
  dispatch: Dispatch<Action | NotificationAction>,
  getState: GetState
): Promise<void> => {
  try {
    const {
      rules: {list},
    } = getState()

    const rule = draftRuleToRule(draftRule)

    const allRuleNames = list.map(r => r.name)

    const clonedName = incrementCloneName(allRuleNames, rule.name)

    const resp = await api.postNotificationRule({
      data: {...rule, name: clonedName},
    })

    if (resp.status !== 201) {
      throw new Error(resp.data.message)
    }

    dispatch(setRule(ruleToDraftRule(resp.data)))

    // add labels?
  } catch (error) {
    console.error(error)
    dispatch(notify(copy.createRuleFailed(error.message)))
  }
}
