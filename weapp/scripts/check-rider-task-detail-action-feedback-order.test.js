const assert = require('assert')
const fs = require('fs')
const path = require('path')

const sourcePath = path.join(__dirname, '..', 'miniprogram', 'pages', 'rider', 'task-detail', 'index.ts')
const source = fs.readFileSync(sourcePath, 'utf8')

assert(
  /function\s+showDeliveryActionFailureFeedback/.test(source),
  'task detail page must keep rider delivery action failure feedback centralized'
)

const feedbackCallIndex = source.indexOf('showDeliveryActionFailureFeedback(err, actionState.actionKey')
assert(
  feedbackCallIndex >= 0,
  'failed rider delivery actions must show the mapped backend business message'
)

const catchStart = source.lastIndexOf('catch (err: unknown) {', feedbackCallIndex)
const finallyStart = source.indexOf('} finally {', feedbackCallIndex)
assert(catchStart >= 0 && finallyStart > catchStart, 'onUpdateStatus must keep an explicit action failure catch block')

const catchBlock = source.slice(catchStart, finallyStart)
const finallyEnd = source.indexOf('\n                }', finallyStart + 1)
assert(finallyEnd > finallyStart, 'onUpdateStatus modal success handler must keep an explicit finally block')

const finallyBlock = source.slice(finallyStart, finallyEnd)

const hideBeforeFeedbackIndex = catchBlock.indexOf('wx.hideLoading()')
const loadingClearedIndex = catchBlock.indexOf('loadingVisible = false')
const feedbackIndex = catchBlock.indexOf('showDeliveryActionFailureFeedback')
assert(
  hideBeforeFeedbackIndex >= 0,
  'action failure catch block must close the loading indicator before showing feedback'
)
assert(
  feedbackIndex >= 0 && hideBeforeFeedbackIndex < feedbackIndex,
  'action failure feedback must be displayed after wx.hideLoading(), otherwise the toast/modal can be swallowed as a silent failure'
)
assert(
  loadingClearedIndex > hideBeforeFeedbackIndex && loadingClearedIndex < feedbackIndex,
  'action failure catch block must mark loading as closed before feedback so finally does not hide the toast/modal again'
)

assert(
  /if\s*\(\s*loadingVisible\s*\)\s*\{[\s\S]*wx\.hideLoading\(\)/.test(finallyBlock),
  'finally must still close loading for successful and reconciled paths'
)
assert(
  finallyBlock.includes('actionLoading: false'),
  'finally must still clear the action loading state'
)

console.log('check-rider-task-detail-action-feedback-order tests passed')
