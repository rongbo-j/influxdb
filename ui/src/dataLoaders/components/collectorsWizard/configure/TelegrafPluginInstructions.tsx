// Libraries
import React, {PureComponent, ChangeEvent} from 'react'
import {connect} from 'react-redux'
import {includes, get} from 'lodash'

// Components
import {Form, Input} from '@influxdata/clockface'
import FancyScrollbar from 'src/shared/components/fancy_scrollbar/FancyScrollbar'
import OnboardingButtons from 'src/onboarding/components/OnboardingButtons'
import PluginsSideBar from 'src/dataLoaders/components/collectorsWizard/configure/PluginsSideBar'

// Actions
import {
  setTelegrafConfigName,
  setTelegrafConfigDescription,
  setActiveTelegrafPlugin,
  setPluginConfiguration,
  createOrUpdateTelegrafConfigAsync,
} from 'src/dataLoaders/actions/dataLoaders'
import {
  incrementCurrentStepIndex,
  decrementCurrentStepIndex,
} from 'src/dataLoaders/actions/steps'
import {notify as notifyAction} from 'src/shared/actions/notifications'

// APIs
import {createDashboardFromTemplate as createDashboardFromTemplateAJAX} from 'src/templates/api'

// Constants
import {
  TelegrafDashboardCreated,
  TelegrafDashboardFailed,
} from 'src/shared/copy/notifications'

// Types
import {AppState, TelegrafPlugin, ConfigurationState} from 'src/types'
import {InputType, ComponentSize} from '@influxdata/clockface'
import {influxdbTemplateList} from 'src/templates/constants/defaultTemplates'

interface DispatchProps {
  onSetTelegrafConfigName: typeof setTelegrafConfigName
  onSetTelegrafConfigDescription: typeof setTelegrafConfigDescription
  onSetActiveTelegrafPlugin: typeof setActiveTelegrafPlugin
  onSetPluginConfiguration: typeof setPluginConfiguration
  onIncrementStep: typeof incrementCurrentStepIndex
  onDecrementStep: typeof decrementCurrentStepIndex
  notify: typeof notifyAction
  onSaveTelegrafConfig: typeof createOrUpdateTelegrafConfigAsync
}

interface StateProps {
  telegrafConfigName: string
  telegrafConfigDescription: string
  telegrafPlugins: TelegrafPlugin[]
  telegrafConfigID: string
  orgID: string
}

type Props = DispatchProps & StateProps

export class TelegrafPluginInstructions extends PureComponent<Props> {
  public render() {
    const {
      telegrafConfigName,
      telegrafConfigDescription,
      telegrafPlugins,
      onDecrementStep,
    } = this.props

    return (
      <Form onSubmit={this.handleFormSubmit} className="data-loading--form">
        <div className="data-loading--scroll-content">
          <div>
            <h3 className="wizard-step--title">Configure Plugins</h3>
            <h5 className="wizard-step--sub-title">
              Configure each plugin from the menu on the left. Some plugins do
              not require any configuration.
            </h5>
          </div>
          <div className="data-loading--columns">
            <PluginsSideBar
              telegrafPlugins={telegrafPlugins}
              onTabClick={this.handleClickSideBarTab}
              title="Plugins"
              visible={this.sideBarVisible}
            />
            <div className="data-loading--column-panel">
              <FancyScrollbar
                autoHide={false}
                className="data-loading--scroll-content"
              >
                <Form.Element label="Telegraf Configuration Name">
                  <Input
                    type={InputType.Text}
                    value={telegrafConfigName}
                    name="name"
                    onChange={this.handleNameInput}
                    titleText="Telegraf Configuration Name"
                    size={ComponentSize.Medium}
                    autoFocus={true}
                  />
                </Form.Element>
                <Form.Element label="Telegraf Configuration Description">
                  <Input
                    type={InputType.Text}
                    value={telegrafConfigDescription}
                    name="description"
                    onChange={this.handleDescriptionInput}
                    titleText="Telegraf Configuration Description"
                    size={ComponentSize.Medium}
                  />
                </Form.Element>
              </FancyScrollbar>
            </div>
          </div>
        </div>

        <OnboardingButtons
          onClickBack={onDecrementStep}
          nextButtonText="Create and Verify"
          className="data-loading--button-container"
        />
      </Form>
    )
  }

  private handleFormSubmit = async () => {
    const {onSaveTelegrafConfig, telegrafConfigID} = this.props

    await onSaveTelegrafConfig()

    if (!telegrafConfigID) {
      this.handleCreateDashboardsForPlugins()
    }

    this.props.onIncrementStep()
  }

  private async handleCreateDashboardsForPlugins() {
    const {notify, telegrafPlugins, orgID} = this.props
    try {
      const configuredPlugins = telegrafPlugins.filter(
        tp => tp.configured === ConfigurationState.Configured
      )

      const configuredPluginTemplateIdentifiers = configuredPlugins
        .map(t => t.templateID)
        .filter(t => t)

      const templatesToInstantiate = influxdbTemplateList.filter(t => {
        return includes(
          configuredPluginTemplateIdentifiers,
          get(t, 'meta.templateID')
        )
      })

      const pendingDashboards = templatesToInstantiate.map(t =>
        createDashboardFromTemplateAJAX(t, orgID)
      )

      const pendingDashboardNames = templatesToInstantiate.map(t =>
        t.meta.name.toLowerCase()
      )

      const dashboards = await Promise.all(pendingDashboards)

      if (dashboards.length) {
        notify(TelegrafDashboardCreated(pendingDashboardNames))
      }
    } catch (err) {
      notify(TelegrafDashboardFailed())
    }
  }

  private get sideBarVisible() {
    const {telegrafPlugins} = this.props

    return telegrafPlugins.length > 0
  }

  private handleNameInput = (e: ChangeEvent<HTMLInputElement>) => {
    this.props.onSetTelegrafConfigName(e.target.value)
  }

  private handleDescriptionInput = (e: ChangeEvent<HTMLInputElement>) => {
    this.props.onSetTelegrafConfigDescription(e.target.value)
  }

  private handleClickSideBarTab = (tabID: string) => {
    const {
      onSetActiveTelegrafPlugin,
      telegrafPlugins,
      onSetPluginConfiguration,
    } = this.props

    const activeTelegrafPlugin = telegrafPlugins.find(tp => tp.active)
    if (!!activeTelegrafPlugin) {
      onSetPluginConfiguration(activeTelegrafPlugin.name)
    }

    onSetActiveTelegrafPlugin(tabID)
  }
}

const mstp = ({
  dataLoading: {
    dataLoaders: {
      telegrafConfigName,
      telegrafConfigDescription,
      telegrafPlugins,
      telegrafConfigID,
    },
  },
  orgs: {org},
}: AppState): StateProps => {
  return {
    telegrafConfigName,
    telegrafConfigDescription,
    telegrafPlugins,
    telegrafConfigID,
    orgID: org.id,
  }
}

const mdtp: DispatchProps = {
  onSetTelegrafConfigName: setTelegrafConfigName,
  onSetTelegrafConfigDescription: setTelegrafConfigDescription,
  onIncrementStep: incrementCurrentStepIndex,
  onDecrementStep: decrementCurrentStepIndex,
  onSetActiveTelegrafPlugin: setActiveTelegrafPlugin,
  onSetPluginConfiguration: setPluginConfiguration,
  onSaveTelegrafConfig: createOrUpdateTelegrafConfigAsync,
  notify: notifyAction,
}

export default connect<StateProps, DispatchProps, {}>(
  mstp,
  mdtp
)(TelegrafPluginInstructions)
