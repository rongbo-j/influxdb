// Libraries
import React, {FC} from 'react'

// Components
import {Context, IconFont} from 'src/clockface'
import {ComponentColor} from '@influxdata/clockface'

interface Props {
  onDelete: () => void
  onClone: () => void
  onEdit: () => void
}

const EndpointCardContext: FC<Props> = ({onDelete, onClone, onEdit}) => {
  return (
    <Context>
      <Context.Menu icon={IconFont.CogThick}>
        <Context.Item
          label="Edit"
          action={onEdit}
          testID="endpoint-card-edit"
        />
      </Context.Menu>
      <Context.Menu icon={IconFont.Duplicate} color={ComponentColor.Secondary}>
        <Context.Item label="Clone" action={onClone} />
      </Context.Menu>
      <Context.Menu
        icon={IconFont.Trash}
        color={ComponentColor.Danger}
        testID="context-delete-menu"
      >
        <Context.Item
          label="Delete"
          action={onDelete}
          testID="context-delete-task"
        />
      </Context.Menu>
    </Context>
  )
}

export default EndpointCardContext
