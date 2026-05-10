import {
  buildBaofuSettlementAccountView,
  getBaofuAccountNextActionText,
  getBaofuAccountStatusText,
  type BaofuAccountOwnerRole,
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
  profileHint: string
  statusView: BaofuSettlementAccountView
  statusText: string
  nextActionText: string
  shouldShowPaymentAction: boolean
  shouldShowProfileAction: boolean
  shouldShowRefreshAction: boolean
  shouldShowOpenReportHint: boolean
  fieldConfigs: BaofuRolePageFieldConfig[]
  profileButtonText: string
  paymentButtonText: string
}

function buildRoleTitle(role: BaofuAccountOwnerRole): string {
  switch (role) {
    case 'merchant':
      return '商户宝付开户'
    case 'operator':
      return '运营商宝付开户'
    default:
      return '骑手宝付开户'
  }
}

function buildVerifyFeePrompt(role: BaofuAccountOwnerRole, feeDisplay: string): string {
  if (role === 'merchant') {
    return '商户开户由平台承担核验费，提交后继续同步状态。'
  }
  return `${feeDisplay} 元核验费由本人支付，支付完成后继续开户。`
}

function buildProfileHint(role: BaofuAccountOwnerRole): string {
  if (role === 'merchant') {
    return '用于商户报备与小程序授权目录绑定。'
  }
  return '用于宝付个人户开户与核验。'
}

function buildFieldConfigs(role: BaofuAccountOwnerRole): BaofuRolePageFieldConfig[] {
  if (role === 'merchant') {
    return [
      { key: 'legal_name', label: '商户主体名称', placeholder: '请输入商户营业执照名称', required: true },
      { key: 'business_license_number', label: '营业执照号', placeholder: '请输入营业执照号', required: true },
      { key: 'legal_person_name', label: '法人姓名', placeholder: '请输入法人姓名', required: true },
      { key: 'legal_person_id_number', label: '法人身份证号', placeholder: '请输入法人身份证号', required: true },
      { key: 'email', label: '联系邮箱', placeholder: '请输入联系邮箱', required: true },
      { key: 'bank_account_no', label: '对公账号', placeholder: '请输入对公账号', required: true },
      { key: 'bank_name', label: '开户银行', placeholder: '请输入开户银行', required: true },
      { key: 'deposit_bank_province', label: '开户地址省份', placeholder: '请输入开户地址省份', required: true },
      { key: 'deposit_bank_city', label: '开户地址城市', placeholder: '请输入开户地址城市', required: true },
      { key: 'deposit_bank_name', label: '开户支行', placeholder: '请输入开户支行', required: true },
      { key: 'contact_name', label: '联系人', placeholder: '请输入联系人', required: false },
      { key: 'contact_mobile', label: '联系人手机号', placeholder: '请输入联系人手机号', required: false }
    ]
  }

  return [
    { key: 'legal_name', label: '姓名', placeholder: '请输入姓名', required: true },
    { key: 'certificate_no', label: '身份证号', placeholder: '请输入身份证号', required: true },
    { key: 'bank_account_no', label: '银行卡号', placeholder: '请输入本人银行卡号', required: true },
    { key: 'bank_mobile', label: '银行预留手机号', placeholder: '请输入银行预留手机号', required: true },
    { key: 'bank_name', label: '开户银行（可选）', placeholder: '如：中国工商银行', required: false }
  ]
}

export function buildBaofuRolePageView(
  role: BaofuAccountOwnerRole,
  response?: BaofuSettlementAccountResponse | null
): BaofuRolePageView {
  const statusView = buildBaofuSettlementAccountView(response)
  const title = buildRoleTitle(role)
  const feeDisplay = `${statusView.verifyFeeDisplay}`
  const shouldShowPaymentAction = role !== 'merchant' && statusView.canStartPayment
  const shouldShowProfileAction = statusView.canSubmitProfile
  const shouldShowRefreshAction = statusView.canRefresh
  const shouldShowOpenReportHint = role === 'merchant' && (statusView.normalizedStatus === 'merchant_report_processing' || statusView.normalizedStatus === 'applet_auth_pending')

  return {
    role,
    title,
    verifyFeePrompt: buildVerifyFeePrompt(role, feeDisplay),
    profileHint: buildProfileHint(role),
    statusView,
    statusText: getBaofuAccountStatusText(statusView.normalizedStatus),
    nextActionText: getBaofuAccountNextActionText(statusView.normalizedStatus, statusView.verifyFeeAmount),
    shouldShowPaymentAction,
    shouldShowProfileAction,
    shouldShowRefreshAction,
    shouldShowOpenReportHint,
    fieldConfigs: buildFieldConfigs(role),
    profileButtonText: role === 'merchant' ? '提交开户资料' : '提交资料并支付核验费',
    paymentButtonText: '继续支付核验费'
  }
}
