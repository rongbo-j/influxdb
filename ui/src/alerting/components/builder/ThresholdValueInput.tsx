// Libraries
import React, {FC} from 'react'

// Components
import {
  FlexBox,
  ComponentSize,
  ComponentStatus,
  AutoComplete,
  Input,
  InputType,
} from '@influxdata/clockface'
import {GreaterThreshold, LesserThreshold} from 'src/types'

// Types
interface Props {
  threshold: GreaterThreshold | LesserThreshold
  changeValue: (value: number) => void
}

const ThresholdValueStatement: FC<Props> = ({threshold, changeValue}) => {
  const onChangeValue = (e: React.ChangeEvent<HTMLInputElement>) => {
    changeValue(Number(e.target.value))
  }
  return (
    <FlexBox.FlexChild
      testID="component-spacer--flex-child"
    >
      <Input
        onChange={onChangeValue}
        name=""
        testID="input-field"
        type={InputType.Number}
        value={threshold.value}
      />
    </FlexBox.FlexChild>
  )
}

export default ThresholdValueStatement
