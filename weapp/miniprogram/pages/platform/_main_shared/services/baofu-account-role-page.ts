import {
  buildBaofuSettlementAccountView,
  getBaofuAccountNextActionText,
  getBaofuAccountStatusText,
  type BaofuAccountOwnerRole,
  type BaofuSettlementAccountPageAction,
  type BaofuSettlementAccountResponse,
  type BaofuSettlementAccountView
} from '../api/baofu-account'

export interface BaofuRolePageFieldConfig {
  key: string
  label: string
  placeholder: string
  type?: string
  required?: boolean
  help?: string
}

export interface BaofuRolePageView {
  role: BaofuAccountOwnerRole
  title: string
  verifyFeePrompt: string
  statusView: BaofuSettlementAccountView
  statusText: string
  nextActionText: string
  primaryAction: BaofuSettlementAccountPageAction
  shouldEnterSubmitDirectly: boolean
  shouldShowPaymentAction: boolean
  shouldShowProfileAction: boolean
  shouldShowOpenReportHint: boolean
  fieldConfigs: BaofuRolePageFieldConfig[]
  profileButtonText: string
  paymentButtonText: string
}

function emptyAction(): BaofuSettlementAccountPageAction {
  return {
    type: 'none',
    text: '',
    theme: 'default'
  }
}

function buildRoleTitle(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '商户宝付开户'
    case 'operator':
      return '运营商宝付开户'
    case 'platform':
      return '平台宝付开户'
    default:
      return '骑手宝付开户'
  }
}

function buildVerifyFeePrompt(role: BaofuAccountOwnerRole, feeDisplay: string): string {
  if (role === 'merchant' || role === 'platform') {
    return '开户核验费由平台承担，提交后继续同步状态。'
  }
  return `${feeDisplay} 元核验费由本人支付，支付完成后继续开户。`
}

function buildFieldConfigs(role: BaofuAccountOwnerRole): BaofuRolePageFieldConfig[] {
  if (role === 'merchant' || role === 'platform') {
    return [
      { key: 'legal_name', label: role === 'platform' ? '平台主体名称' : '商户主体名称', placeholder: '', required: true },
      { key: 'business_license_number', label: '营业执照号', placeholder: '', required: true },
      { key: 'legal_person_name', label: '法人姓名', placeholder: '', required: true },
      { key: 'legal_person_id_number', label: '法人身份证号', placeholder: '18位身份证号', required: true },
      { key: 'email', label: '联系邮箱', placeholder: 'name@example.com', required: true },
      { key: 'bank_account_no', label: '对公账号', placeholder: '', required: true },
      { key: 'bank_name', label: '开户银行', placeholder: '', required: true },
      { key: 'deposit_bank_province', label: '开户省份', placeholder: '', required: true },
      { key: 'deposit_bank_city', label: '开户城市', placeholder: '', required: true },
      { key: 'deposit_bank_name', label: '开户支行', placeholder: '', required: true }
    ]
  }

  return [
    { key: 'legal_name', label: '姓名', placeholder: '', required: true },
    { key: 'certificate_no', label: '身份证号', placeholder: '18位身份证号', required: true },
    { key: 'bank_account_no', label: '银行卡号', placeholder: '本人借记卡号', required: true },
    { key: 'bank_mobile', label: '预留手机号', placeholder: '11位手机号', required: true }
  ]
}

function buildPrimaryAction(
  role: BaofuAccountOwnerRole,
  statusView: BaofuSettlementAccountView
): BaofuSettlementAccountPageAction {
  if (statusView.canSubmitProfile) {
    return {
      type: 'submit_profile',
      text: role === 'merchant' || role === 'platform' ? '提交开户资料' : '提交资料并支付核验费',
      theme: 'primary'
    }
  }

  if (role !== 'merchant' && role !== 'platform' && statusView.canContinuePayment) {
    return {
      type: 'continue_payment',
      text: '继续支付核验费',
      theme: 'primary'
    }
  }

  if (statusView.canRefreshStatus) {
    return emptyAction()
  }

  return emptyAction()
}

export function buildBaofuRolePageView(
  role: BaofuAccountOwnerRole,
  response?: BaofuSettlementAccountResponse | null
): BaofuRolePageView {
  const statusView = buildBaofuSettlementAccountView(response)
  const title = buildRoleTitle(role)
  const feeDisplay = `${statusView.verifyFeeDisplay}`
  const shouldShowPaymentAction = role !== 'merchant' && role !== 'platform' && statusView.canStartPayment
  const platformSubmitDisabled = role === 'platform'
  const shouldShowProfileAction = statusView.canSubmitProfile && !platformSubmitDisabled
  const shouldShowOpenReportHint = role === 'merchant' && (statusView.normalizedStatus === 'merchant_report_processing' || statusView.normalizedStatus === 'applet_auth_pending')
  const primaryAction = platformSubmitDisabled && statusView.canSubmitProfile ? emptyAction() : buildPrimaryAction(role, statusView)
  const shouldEnterSubmitDirectly = statusView.normalizedStatus === 'profile_pending' && !platformSubmitDisabled

  return {
    role,
    title,
    verifyFeePrompt: buildVerifyFeePrompt(role, feeDisplay),
    statusView,
    statusText: getBaofuAccountStatusText(statusView.normalizedStatus),
    nextActionText: getBaofuAccountNextActionText(statusView.normalizedStatus, statusView.verifyFeeAmount),
    primaryAction,
    shouldEnterSubmitDirectly,
    shouldShowPaymentAction,
    shouldShowProfileAction,
    shouldShowOpenReportHint,
    fieldConfigs: buildFieldConfigs(role),
    profileButtonText: role === 'merchant' || role === 'platform' ? '提交开户资料' : '提交资料并支付核验费',
    paymentButtonText: '继续支付核验费'
  }
}
