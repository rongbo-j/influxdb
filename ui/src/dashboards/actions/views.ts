// Utils
import {getView as getViewFromState} from 'src/dashboards/selectors'

// APIs
import {
  getView as getViewAJAX,
  updateView as updateViewAJAX,
} from 'src/dashboards/apis/'

// Constants
import * as copy from 'src/shared/copy/notifications'

// Actions
import {
  notify,
  Action as NotificationAction,
} from 'src/shared/actions/notifications'
import {setActiveTimeMachine} from 'src/timeMachine/actions'

// Types
import {RemoteDataState, QueryView, GetState} from 'src/types'
import {Dispatch} from 'redux'
import {View} from 'src/types'
import {Action as TimeMachineAction} from 'src/timeMachine/actions'
import {TimeMachineID} from 'src/types'

export type Action = SetViewAction | SetViewsAction | ResetViewsAction

export interface SetViewsAction {
  type: 'SET_VIEWS'
  payload: {
    views?: View[]
    status: RemoteDataState
  }
}

export const setViews = (
  status: RemoteDataState,
  views: View[]
): SetViewsAction => ({
  type: 'SET_VIEWS',
  payload: {views, status},
})

export interface SetViewAction {
  type: 'SET_VIEW'
  payload: {
    id: string
    view: View
    status: RemoteDataState
  }
}

export const setView = (
  id: string,
  view: View,
  status: RemoteDataState
): SetViewAction => ({
  type: 'SET_VIEW',
  payload: {id, view, status},
})

export interface ResetViewsAction {
  type: 'RESET_VIEWS'
}

export const resetViews = (): ResetViewsAction => ({
  type: 'RESET_VIEWS',
})

export const getView = (dashboardID: string, cellID: string) => async (
  dispatch: Dispatch<Action>
): Promise<void> => {
  dispatch(setView(cellID, null, RemoteDataState.Loading))
  try {
    const view = await getViewAJAX(dashboardID, cellID)

    dispatch(setView(cellID, view, RemoteDataState.Done))
  } catch {
    dispatch(setView(cellID, null, RemoteDataState.Error))
  }
}

export const updateView = (dashboardID: string, view: View) => async (
  dispatch: Dispatch<Action>
): Promise<View> => {
  const viewID = view.cellID

  dispatch(setView(viewID, view, RemoteDataState.Loading))

  try {
    const newView = await updateViewAJAX(dashboardID, viewID, view)

    dispatch(setView(viewID, newView, RemoteDataState.Done))

    return newView
  } catch {
    dispatch(setView(viewID, null, RemoteDataState.Error))
  }
}

export const getViewForTimeMachine = (
  dashboardID: string,
  cellID: string,
  timeMachineID: TimeMachineID
) => async (
  dispatch: Dispatch<Action | TimeMachineAction | NotificationAction>,
  getState: GetState
): Promise<void> => {
  const state = getState()
  dispatch(setView(cellID, null, RemoteDataState.Loading))
  try {
    let view = getViewFromState(state, cellID) as QueryView

    if (!view) {
      view = (await getViewAJAX(dashboardID, cellID)) as QueryView
    }

    dispatch(setActiveTimeMachine(timeMachineID, {view}))
  } catch (e) {
    dispatch(notify(copy.getViewFailed(e.message)))
    dispatch(setView(cellID, null, RemoteDataState.Error))
  }
}
