// Libraries
import {binaryPrefixFormatter} from '@influxdata/giraffe'

// Types
import {Notification} from 'src/types'
import {NotificationStyle} from 'src/types/notifications'

// Constants
import {FIVE_SECONDS, TEN_SECONDS, INFINITE} from 'src/shared/constants/index'
import {QUICKSTART_SCRAPER_TARGET_URL} from 'src/dataLoaders/constants/pluginConfigs'
import {QUICKSTART_DASHBOARD_NAME} from 'src/onboarding/constants/index'

const bytesFormatter = binaryPrefixFormatter({
  suffix: 'B',
  significantDigits: 2,
  trimZeros: true,
})

type NotificationExcludingMessage = Pick<
  Notification,
  Exclude<keyof Notification, 'message'>
>

const defaultErrorNotification: NotificationExcludingMessage = {
  style: NotificationStyle.Error,
  icon: 'alert-triangle',
  duration: TEN_SECONDS,
}

const defaultSuccessNotification: NotificationExcludingMessage = {
  style: NotificationStyle.Success,
  icon: 'checkmark',
  duration: FIVE_SECONDS,
}

const defaultDeletionNotification: NotificationExcludingMessage = {
  style: NotificationStyle.Primary,
  icon: 'trash',
  duration: FIVE_SECONDS,
}

//  Misc Notifications
//  ----------------------------------------------------------------------------

export const newVersion = (version: string): Notification => ({
  style: NotificationStyle.Info,
  icon: 'cubo-uniform',
  duration: INFINITE,
  message: `Welcome to the latest Chronograf${version}. Local settings cleared.`,
})

export const loadLocalSettingsFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Loading local settings failed: ${error}`,
})

export const presentationMode = (): Notification => ({
  style: NotificationStyle.Primary,
  icon: 'expand-b',
  duration: 7500,
  message: 'Press ESC to exit Presentation Mode.',
})

export const sessionTimedOut = (): Notification => ({
  style: NotificationStyle.Primary,
  icon: 'triangle',
  duration: INFINITE,
  message: 'Your session has timed out. Log in again to continue.',
})

export const resultTooLarge = (bytesRead: number): Notification => ({
  style: NotificationStyle.Error,
  icon: 'triangle',
  duration: FIVE_SECONDS,
  message: `Large response truncated to first ${bytesFormatter(bytesRead)}`,
})

// Onboarding notifications
export const SetupSuccess: Notification = {
  ...defaultSuccessNotification,
  message: 'Initial user details have been successfully set',
}

export const SetupError = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Could not set up admin user: ${message}`,
})

export const SigninError: Notification = {
  ...defaultErrorNotification,
  message: `Could not sign in`,
}

export const QuickstartScraperCreationSuccess: Notification = {
  ...defaultSuccessNotification,
  message: `The InfluxDB Scraper has been configured for ${QUICKSTART_SCRAPER_TARGET_URL}`,
}

export const QuickstartScraperCreationError: Notification = {
  ...defaultErrorNotification,
  message: `Failed to configure InfluxDB Scraper`,
}

export const QuickstartDashboardCreationSuccess: Notification = {
  ...defaultSuccessNotification,
  message: `The ${QUICKSTART_DASHBOARD_NAME} Dashboard has been created`,
}

export const QuickstartDashboardCreationError: Notification = {
  ...defaultErrorNotification,
  message: `Failed to create ${QUICKSTART_DASHBOARD_NAME} Dashboard`,
}

export const TelegrafConfigCreationSuccess: Notification = {
  ...defaultSuccessNotification,
  message: `Your configurations have been saved`,
}

export const TelegrafConfigCreationError: Notification = {
  ...defaultErrorNotification,
  message: `Failed to save configurations`,
}

//  Task Notifications
//  ----------------------------------------------------------------------------
export const addTaskLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to add label to task',
})

export const removeTaskLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to remove label from task',
})

//  Dashboard Notifications

export const dashboardGetFailed = (dashboardID: string): Notification => ({
  ...defaultErrorNotification,
  icon: 'dash-h',
  message: `Failed to load dashboard with id "${dashboardID}"`,
})

export const dashboardUpdateFailed = (): Notification => ({
  ...defaultErrorNotification,
  icon: 'dash-h',
  message: 'Could not update dashboard',
})

export const dashboardDeleted = (name: string): Notification => ({
  ...defaultSuccessNotification,
  icon: 'dash-h',
  message: `Dashboard ${name} deleted successfully.`,
})

export const dashboardCreateFailed = () => ({
  ...defaultErrorNotification,
  message: 'Failed to create dashboard.',
})

export const dashboardDeleteFailed = (
  name: string,
  errorMessage: string
): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete Dashboard ${name}: ${errorMessage}.`,
})

export const cellAdded = (
  cellName?: string,
  dashboardName?: string
): Notification => ({
  ...defaultSuccessNotification,
  icon: 'dash-h',
  message: `Added new cell ${cellName + ' '}to dashboard ${dashboardName}`,
})

export const cellAddFailed = (
  cellName?: string,
  dashboardName?: string
): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to add cell ${cellName + ' '}to dashboard ${dashboardName}`,
})

export const cellUpdateFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update cell`,
})

export const cellDeleted = (): Notification => ({
  ...defaultDeletionNotification,
  icon: 'dash-h',
  duration: 1900,
  message: `Cell deleted from dashboard.`,
})

export const addDashboardLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to add label to dashboard',
})

export const removedDashboardLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to remove label from dashboard',
})

// Variables & URL Queries
export const invalidTimeRangeValueInURLQuery = (): Notification => ({
  ...defaultErrorNotification,
  icon: 'cube',
  message: `Invalid URL query value supplied for lower or upper time range.`,
})

export const invalidZoomedTimeRangeValueInURLQuery = (): Notification => ({
  ...defaultErrorNotification,
  icon: 'cube',
  message: `Invalid URL query value supplied for zoomed lower or zoomed upper time range.`,
})

export const getVariablesFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to fetch variables',
})

export const getVariableFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to fetch variable',
})

export const createVariableFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  icon: 'cube',
  message: `Failed to create variable: ${error}`,
})

export const createVariableSuccess = (name: string): Notification => ({
  ...defaultSuccessNotification,
  icon: 'cube',
  message: `Successfully created new variable: ${name}.`,
})

export const deleteVariableFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  icon: 'cube',
  message: `Failed to delete variable: ${error}`,
})

export const deleteVariableSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  icon: 'cube',
  message: 'Successfully deleted the variable',
})

export const updateVariableFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  icon: 'cube',
  message: `Failed to update variable: ${error}`,
})

export const updateVariableSuccess = (name: string): Notification => ({
  ...defaultSuccessNotification,
  icon: 'cube',
  message: `Successfully updated variable: ${name}.`,
})

export const copyToClipboardSuccess = (
  text: string,
  title: string = ''
): Notification => ({
  ...defaultSuccessNotification,
  icon: 'dash-h',
  type: 'copyToClipboardSuccess',
  message: `${title} '${text}' has been copied to clipboard.`,
})

export const copyToClipboardFailed = (
  text: string,
  title: string = ''
): Notification => ({
  ...defaultErrorNotification,
  message: `${title}'${text}' was not copied to clipboard.`,
})

// Templates
export const addTemplateLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to add label to template',
})

export const removeTemplateLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to remove label from template',
})

export const TelegrafDashboardCreated = (configs: string[]): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully created dashboards for telegraf plugin${
    configs.length > 1 ? 's' : ''
  }: ${configs.join(', ')}.`,
})

export const TelegrafDashboardFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Could not create dashboards for one or more plugins`,
})

export const importTaskSucceeded = (): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully imported task.`,
})

export const importTaskFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to import task: ${error}`,
})

export const importDashboardSucceeded = (): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully imported dashboard.`,
})

export const importDashboardFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to import dashboard: ${error}`,
})

export const importTemplateSucceeded = (): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully imported template.`,
})

export const importTemplateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to import template: ${error}`,
})

export const createTemplateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to  resource as template: ${error}`,
})

export const createResourceFromTemplateFailed = (
  error: string
): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create from template: ${error}`,
})

export const updateTemplateSucceeded = (): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully updated template.`,
})

export const updateTemplateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update template: ${error}`,
})

export const deleteTemplateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete template: ${error}`,
})

export const deleteTemplateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Template was deleted successfully',
})

export const cloneTemplateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to clone template: ${error}`,
})

export const cloneTemplateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Template cloned successfully',
})

export const resourceSavedAsTemplate = (
  resourceName: string
): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully saved ${resourceName.toLowerCase()} as template.`,
})

export const saveResourceAsTemplateFailed = (
  resourceName: string,
  error: string
): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to save ${resourceName.toLowerCase()} as template: ${error}`,
})

// Labels
export const getLabelsFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to fetch labels',
})

export const createLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to create label',
})

export const updateLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to update label',
})

export const deleteLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to delete label',
})

// Buckets
export const getBucketsFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to fetch buckets',
})

// Limits
export const writeLimitReached = (): Notification => ({
  ...defaultErrorNotification,
  message: `Exceeded write limits.`,
  duration: FIVE_SECONDS,
  type: 'writeLimitReached',
})

export const readLimitReached = (): Notification => ({
  ...defaultErrorNotification,
  message: `Exceeded query limits.`,
  duration: FIVE_SECONDS,
  type: 'readLimitReached',
})

export const rateLimitReached = (secs?: number): Notification => {
  const retryText = ` Please try again in ${secs} seconds`
  return {
    ...defaultErrorNotification,
    message: `Exceeded rate limits.${secs ? retryText : ''} `,
    duration: FIVE_SECONDS,
    type: 'rateLimitReached',
  }
}

export const resourceLimitReached = (resourceName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Oops. It looks like you have reached the maximum number of ${resourceName} allowed as part of your plan. If you would like to upgrade and remove this restriction, reach out to cloudbeta@influxdata.com.`,
  duration: FIVE_SECONDS,
  type: 'resourceLimitReached',
})

export const taskNotCreated = (additionalMessage: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create new task: ${additionalMessage}`,
})

export const taskCreatedSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'New task created successfully',
})

export const taskNotFound = (additionalMessage: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to find task: ${additionalMessage}`,
})

export const tasksFetchFailed = (additionalMessage: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get tasks from server: ${additionalMessage}`,
})

export const taskDeleteFailed = (additionalMessage: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete task: ${additionalMessage}`,
})

export const taskDeleteSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Task was deleted successfully',
})

export const taskCloneSuccess = (taskName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Successfully cloned task ${taskName}`,
})

export const taskCloneFailed = (
  taskName: string,
  additionalMessage: string
): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to clone task ${taskName}: ${additionalMessage} `,
})

export const taskUpdateFailed = (additionalMessage: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update task: ${additionalMessage}`,
})

export const taskUpdateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Task was updated successfully',
})

export const taskImportFailed = (errorMessage: string): Notification => ({
  ...defaultErrorNotification,
  duration: INFINITE,
  message: `Failed to import Task: ${errorMessage}.`,
})

export const taskImportSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  duration: FIVE_SECONDS,
  message: `Successfully imported task.`,
})

export const taskRunSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  duration: FIVE_SECONDS,
  message: 'Task scheduled successfully',
})

export const taskRunFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  duration: FIVE_SECONDS,
  message: `Failed to run task: ${error}`,
})

export const taskGetFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  duration: FIVE_SECONDS,
  message: `Failed to get runs: ${error}`,
})

export const getTelegrafConfigFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to get telegraf config',
})

export const savingNoteFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  duration: FIVE_SECONDS,
  message: `Failed to save note: ${error}`,
})

export const bucketDeleteFailed = (bucketName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete bucket: "${bucketName}"`,
})

export const bucketCreateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Bucket was successfully created',
})

export const bucketCreateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create bucket: ${error}`,
})

export const bucketUpdateSuccess = (bucketName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Bucket "${bucketName}" was successfully updated`,
})

export const bucketUpdateFailed = (error: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update bucket: "${error}"`,
})

export const bucketRenameSuccess = (bucketName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Bucket was successfully renamed "${bucketName}"`,
})

export const bucketRenameFailed = (bucketName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to rename bucket "${bucketName}"`,
})

export const orgCreateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Organization was successfully created',
})

export const orgCreateFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to create organization',
})

export const orgEditSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Organization was successfully updated',
})

export const orgEditFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to update organization',
})

export const orgRenameSuccess = (orgName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Organization was successfully renamed "${orgName}"`,
})

export const orgRenameFailed = (orgName): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update organization "${orgName}"`,
})

export const scraperDeleteSuccess = (scraperName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Scraper "${scraperName}" was successfully deleted`,
})

export const scraperDeleteFailed = (scraperName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete scraper: "${scraperName}"`,
})

export const scraperCreateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Scraper was created successfully',
})

export const scraperCreateFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to create scraper',
})

export const scraperUpdateSuccess = (scraperName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Scraper "${scraperName}" was updated successfully`,
})

export const scraperUpdateFailed = (scraperName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update scraper: "${scraperName}"`,
})

export const telegrafGetFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to get telegraf configs',
})

export const telegrafCreateFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to create telegraf',
})

export const telegrafUpdateFailed = (telegrafName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update telegraf: "${telegrafName}"`,
})

export const addTelelgrafLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to add label to telegraf config`,
})

export const removeTelelgrafLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to remove label from telegraf config`,
})

export const authorizationsGetFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to get tokens',
})

export const authorizationCreateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Token was created successfully',
})

export const authorizationCreateFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: 'Failed to create tokens',
})

export const authorizationUpdateSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Token was updated successfully',
})

export const authorizationUpdateFailed = (desc: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update token: "${desc}"`,
})

export const authorizationDeleteSuccess = (): Notification => ({
  ...defaultSuccessNotification,
  message: 'Token was deleted successfully',
})

export const authorizationDeleteFailed = (desc: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete token: "${desc}"`,
})

export const telegrafDeleteSuccess = (telegrafName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Telegraf "${telegrafName}" was deleted successfully`,
})

export const telegrafDeleteFailed = (telegrafName: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete telegraf: "${telegrafName}"`,
})

export const memberAddSuccess = (username: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Member "${username}" was added successfully`,
})

export const memberAddFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to add members: "${message}"`,
})

export const memberRemoveSuccess = (memberName: string): Notification => ({
  ...defaultSuccessNotification,
  message: `Member "${memberName}" was removed successfully`,
})

export const memberRemoveFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to remove members: "${message}"`,
})

export const addVariableLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to add label to variables`,
})

export const removeVariableLabelFailed = (): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to remove label from variables`,
})

export const invalidMapType = (): Notification => ({
  ...defaultErrorNotification,
  message: `Variables of type map accept two comma separated values per line`,
})

export const getChecksFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get checks: ${message}`,
})

export const getCheckFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get check: ${message}`,
})

export const getNotificationRulesFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get notification rules: ${message}`,
})

export const getNotificationRuleFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get notification rule: ${message}`,
})

export const createCheckFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create check: ${message}`,
})

export const updateCheckFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update check: ${message}`,
})

export const deleteCheckFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete check: ${message}`,
})

export const createRuleFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create notification rule: ${message}`,
})

export const updateRuleFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update notification rule: ${message}`,
})

export const deleteRuleFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete notification rule: ${message}`,
})

export const getViewFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to load resources for cell: ${message}`,
})

export const getEndpointFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get endpoints: ${message}`,
})

export const getEndpointsFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to get endoint: ${message}`,
})

export const createEndpointFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to create endpoint: ${message}`,
})

export const updateEndpointFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to update endpoint: ${message}`,
})

export const deleteEndpointFailed = (message: string): Notification => ({
  ...defaultErrorNotification,
  message: `Failed to delete endpoint: ${message}`,
})
