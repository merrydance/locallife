import {
  buildMerchantApplymentStatusView,
  getMerchantApplymentStatus,
  type ApplymentStatusResponse,
  type MerchantApplymentAccountValidationView,
  type MerchantApplymentStatusView
} from '../api/merchant-applyment'

export type MerchantApplymentStage =
  | 'submit_required'
  | 'action_required'
  | 'reviewing'
  | 'rejected'
  | 'opened'

export type MerchantApplymentTaskType =
  | 'submit_material'
  | 'sign_agreement'
  | 'legal_validation'
  | 'bank_transfer_validation'
  | 'wait_review'
  | 'resubmit_after_reject'
  | 'view_settlement'
  | 'none'

export type MerchantApplymentTaskIntent = 'navigate' | 'refresh' | 'inline' | 'none'

export type MerchantApplymentResultState =
  | 'action_required'
  | 'processing'
  | 'failed'
  | 'completed'
  | 'unknown'

export type MerchantApplymentReentryPolicy = 'force_refresh_on_show' | 'refresh_within_window'

export interface MerchantApplymentWorkflowTask {
  type: MerchantApplymentTaskType
  title: string
  description: string
  actionText: string
  actionIntent: MerchantApplymentTaskIntent
  actionPath: string
}

export interface MerchantApplymentWorkflowSecondaryTask {
  type: MerchantApplymentTaskType | 'refresh_status' | 'copy_validation_account' | 'copy_validation_remark'
  label: string
  actionIntent: MerchantApplymentTaskIntent
  actionPath: string
  value?: string
}

export interface MerchantApplymentWorkflowView {
  status: ApplymentStatusResponse
  statusView: MerchantApplymentStatusView
  currentStage: MerchantApplymentStage
  currentTask: MerchantApplymentWorkflowTask
  secondaryTasks: MerchantApplymentWorkflowSecondaryTask[]
  resultState: MerchantApplymentResultState
  reentryPolicy: MerchantApplymentReentryPolicy
  headline: string
  summary: string
  stageTitle: string
  stageDescription: string
  primaryActionText: string
  primaryActionIntent: MerchantApplymentTaskIntent
  primaryActionPath: string
  statusItems: Array<{ label: string, value: string }>
  accountValidation: MerchantApplymentAccountValidationView | null
  currentTaskQRCodeValue: string
  currentTaskQRCodeHint: string
}

const SUBMIT_PAGE_PATH = '/pages/merchant/settings/applyment/submit/index'
const ACTION_PAGE_PATH = '/pages/merchant/settings/applyment/action/index'
const APPLYMENT_HOME_PAGE_PATH = '/pages/merchant/settings/applyment/index'

function buildActionTaskPath(taskType?: MerchantApplymentTaskType) {
  return taskType && taskType !== 'none'
    ? `${ACTION_PAGE_PATH}?task=${taskType}`
    : ACTION_PAGE_PATH
}

function isResubmittable(statusView: MerchantApplymentStatusView) {
  return statusView.normalizedStatus === 'rejected' || statusView.normalizedStatus === 'cancelled'
}

function resolveCurrentStage(statusView: MerchantApplymentStatusView): MerchantApplymentStage {
  if (statusView.isOpened) {
    return 'opened'
  }

  if (!statusView.hasApplyment) {
    return 'submit_required'
  }

  if (statusView.normalizedStatus === 'rejected' || statusView.normalizedStatus === 'cancelled' || statusView.normalizedStatus === 'frozen') {
    return 'rejected'
  }

  if (statusView.needsAccountValidation || statusView.needsSign) {
    return 'action_required'
  }

  if (statusView.canSubmitOpenInfo) {
    return 'submit_required'
  }

  return 'reviewing'
}

function isRequestedActionTaskAvailable(statusView: MerchantApplymentStatusView, taskType?: MerchantApplymentTaskType) {
  switch (taskType) {
    case 'legal_validation':
      return statusView.needsLegalValidation
    case 'bank_transfer_validation':
      return statusView.needsAccountValidation
    case 'sign_agreement':
      return statusView.needsSign
    default:
      return false
  }
}

function buildRequestedActionTask(statusView: MerchantApplymentStatusView, taskType?: MerchantApplymentTaskType): MerchantApplymentWorkflowTask | null {
  if (!isRequestedActionTaskAvailable(statusView, taskType)) {
    return null
  }

  switch (taskType) {
    case 'legal_validation':
      return {
        type: 'legal_validation',
        title: '先完成法人验证',
        description: '保存二维码后用法人微信扫一扫，并按微信提示完成。',
        actionText: '查看二维码',
        actionIntent: 'navigate',
        actionPath: buildActionTaskPath('legal_validation')
      }
    case 'bank_transfer_validation':
      return {
        type: 'bank_transfer_validation',
        title: '先完成账户验证',
        description: '请按微信支付提供的信息完成验证。',
        actionText: '去完成账户验证',
        actionIntent: 'navigate',
        actionPath: buildActionTaskPath('bank_transfer_validation')
      }
    case 'sign_agreement':
      return {
        type: 'sign_agreement',
        title: '完成微信待办',
        description: '保存二维码后用微信扫一扫，从相册选择二维码并按微信提示完成。',
        actionText: '查看二维码',
        actionIntent: 'navigate',
        actionPath: buildActionTaskPath('sign_agreement')
      }
    default:
      return null
  }
}

function buildCurrentTask(
  statusView: MerchantApplymentStatusView,
  currentStage: MerchantApplymentStage,
  preferredTaskType?: MerchantApplymentTaskType
): MerchantApplymentWorkflowTask {
  if (currentStage === 'opened') {
    return {
      type: 'none',
      title: '收款能力已开通',
      description: '微信支付已开通。',
      actionText: '返回开户首页',
      actionIntent: 'navigate',
      actionPath: APPLYMENT_HOME_PAGE_PATH
    }
  }

  if (currentStage === 'rejected') {
    return {
      type: isResubmittable(statusView) ? 'resubmit_after_reject' : 'none',
      title: statusView.normalizedStatus === 'frozen' ? '当前账户不可继续开户' : '根据审核结果重新提交资料',
      description: statusView.showRejectReason
        ? `拒绝原因：${statusView.rejectReason}`
        : (statusView.blockReason || statusView.statusDesc),
      actionText: isResubmittable(statusView) ? '重新填写资料' : '返回首页',
      actionIntent: isResubmittable(statusView) ? 'navigate' : 'none',
      actionPath: isResubmittable(statusView) ? SUBMIT_PAGE_PATH : ''
    }
  }

  if (currentStage === 'submit_required') {
    return {
      type: 'submit_material',
      title: '先提交开户资料',
      description: '填写结算账户资料后提交开户申请。',
      actionText: statusView.submitActionLabel,
      actionIntent: 'navigate',
      actionPath: SUBMIT_PAGE_PATH
    }
  }

  const requestedTask = buildRequestedActionTask(statusView, preferredTaskType)
  if (requestedTask) {
    return requestedTask
  }

  if (statusView.needsLegalValidation) {
    return {
      type: 'legal_validation',
      title: '先完成法人验证',
      description: '保存二维码后用法人微信扫一扫，并按微信提示完成。',
      actionText: '查看二维码',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('legal_validation')
    }
  }

  if (statusView.needsAccountValidation) {
    return {
      type: 'bank_transfer_validation',
      title: '先完成账户验证',
      description: '请按微信支付提供的信息完成验证。',
      actionText: '去完成账户验证',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('bank_transfer_validation')
    }
  }

  if (statusView.needsSign) {
    return {
      type: 'sign_agreement',
      title: '完成微信待办',
      description: '保存二维码后用微信扫一扫，从相册选择二维码并按微信提示完成。',
      actionText: '查看二维码',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('sign_agreement')
    }
  }

  return {
    type: 'wait_review',
    title: '等待微信审核',
    description: statusView.statusDesc || '微信支付正在审核开户资料，审核期间无需重复提交。',
    actionText: '刷新最新状态',
    actionIntent: 'refresh',
    actionPath: ''
  }
}

function buildSecondaryTasks(
  statusView: MerchantApplymentStatusView,
  currentTask: MerchantApplymentWorkflowTask,
  currentStage: MerchantApplymentStage
): MerchantApplymentWorkflowSecondaryTask[] {
  const tasks: MerchantApplymentWorkflowSecondaryTask[] = []

  if (statusView.hasApplyment && currentStage !== 'opened' && currentStage !== 'action_required') {
    tasks.push({
      type: 'refresh_status',
      label: '刷新开户状态',
      actionIntent: 'refresh',
      actionPath: ''
    })
  }

  if (statusView.needsSign && currentTask.type !== 'sign_agreement') {
    tasks.push({
      type: 'sign_agreement',
      label: '查看微信待办',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('sign_agreement')
    })
  }

  if (statusView.needsLegalValidation && currentTask.type !== 'legal_validation') {
    tasks.push({
      type: 'legal_validation',
      label: '查看法人验证',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('legal_validation')
    })
  }

  if (statusView.needsAccountValidation && currentTask.type !== 'bank_transfer_validation') {
    tasks.push({
      type: 'bank_transfer_validation',
      label: '查看账户验证',
      actionIntent: 'navigate',
      actionPath: buildActionTaskPath('bank_transfer_validation')
    })
  }

  if (statusView.accountValidation?.destinationAccountNumber && statusView.accountValidation.destinationAccountNumber !== '-') {
    tasks.push({
      type: 'copy_validation_account',
      label: '复制收款卡号',
      actionIntent: 'inline',
      actionPath: '',
      value: statusView.accountValidation.destinationAccountNumber
    })
  }

  if (statusView.accountValidation?.remark && statusView.accountValidation.remark !== '-') {
    tasks.push({
      type: 'copy_validation_remark',
      label: '复制汇款备注',
      actionIntent: 'inline',
      actionPath: '',
      value: statusView.accountValidation.remark
    })
  }

  return tasks
}

function buildStageDescription(statusView: MerchantApplymentStatusView, currentTask: MerchantApplymentWorkflowTask, currentStage: MerchantApplymentStage) {
  switch (currentStage) {
    case 'submit_required':
      return '主体审核通过后，先补齐开户资料，再进入微信处理阶段。'
    case 'action_required':
      return currentTask.description
    case 'reviewing':
      return '微信支付正在审核资料；此阶段只保留状态回查，不再重复提交。'
    case 'rejected':
      return currentTask.description
    case 'opened':
      return '微信支付已开通。'
    default:
      return statusView.statusDesc
  }
}

function buildStatusItems(statusView: MerchantApplymentStatusView, currentStage: MerchantApplymentStage) {
  const items = [
    { label: '当前阶段', value: statusView.guideTitle || '-' },
    { label: '状态说明', value: statusView.statusDesc || '-' }
  ]

  if (statusView.signStateText) {
    items.push({ label: '签约状态', value: statusView.signStateText })
  }

  if (currentStage === 'rejected' && statusView.showRejectReason) {
    items.push({ label: '拒绝原因', value: statusView.rejectReason })
  }

  return items
}

export function buildMerchantApplymentWorkflowView(
  status: ApplymentStatusResponse | null,
  preferredTaskType?: MerchantApplymentTaskType
): MerchantApplymentWorkflowView {
  const statusView = buildMerchantApplymentStatusView(status)
  const currentStage = resolveCurrentStage(statusView)
  const currentTask = buildCurrentTask(statusView, currentStage, preferredTaskType)
  const secondaryTasks = buildSecondaryTasks(statusView, currentTask, currentStage)
  const resultState: MerchantApplymentResultState = currentStage === 'opened'
    ? 'completed'
    : currentStage === 'rejected'
      ? 'failed'
      : currentStage === 'action_required'
        ? 'action_required'
        : currentStage === 'reviewing'
          ? 'processing'
          : 'unknown'

  const stageTitleMap: Record<MerchantApplymentStage, string> = {
    submit_required: '资料提交阶段',
    action_required: '微信待办阶段',
    reviewing: '审核结果阶段',
    rejected: '审核结果阶段',
    opened: '开通完成阶段'
  }

  const headline = currentTask.title
  const summary = currentStage === 'opened'
    ? '收款能力已开通。'
    : currentStage === 'reviewing'
      ? '资料已提交，当前以微信审核结果为准。'
      : currentTask.description

  return {
    status: status || { status: 'not_applied', status_desc: '' },
    statusView,
    currentStage,
    currentTask,
    secondaryTasks,
    resultState,
    reentryPolicy: currentStage === 'action_required' ? 'force_refresh_on_show' : 'refresh_within_window',
    headline,
    summary,
    stageTitle: stageTitleMap[currentStage],
    stageDescription: buildStageDescription(statusView, currentTask, currentStage),
    primaryActionText: currentTask.actionText,
    primaryActionIntent: currentTask.actionIntent,
    primaryActionPath: currentTask.actionPath,
    statusItems: buildStatusItems(statusView, currentStage),
    accountValidation: statusView.accountValidation,
    currentTaskQRCodeValue: currentTask.type === 'sign_agreement'
      ? statusView.signURL
      : currentTask.type === 'legal_validation'
        ? statusView.legalValidationURL
        : '',
    currentTaskQRCodeHint: currentTask.type === 'sign_agreement'
      ? '保存后用微信扫一扫，从相册选择二维码并按微信提示完成。'
      : currentTask.type === 'legal_validation'
        ? '保存后用法人微信扫一扫，从相册选择二维码并按微信提示完成。'
        : ''
  }
}

export async function fetchMerchantApplymentWorkflowView(preferredTaskType?: MerchantApplymentTaskType) {
  const status = await getMerchantApplymentStatus()
  return buildMerchantApplymentWorkflowView(status, preferredTaskType)
}
