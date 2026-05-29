import { AgreementService, type AgreementBrief } from './agreement'

export interface AgreementConsentPayload {
  user_agreement_version: string
  privacy_policy_version: string
  consented_at: string
}

function getVersionByType(items: AgreementBrief[], type: string): string {
  const matched = items.find((item) => item.type === type)
  return matched?.version || ''
}

export async function buildAgreementConsentPayload(): Promise<AgreementConsentPayload> {
  const agreements = await AgreementService.listActiveAgreements()
  const userAgreementVersion = getVersionByType(agreements, 'USER_AGREEMENT')
  const privacyPolicyVersion = getVersionByType(agreements, 'PRIVACY_POLICY')

  if (!userAgreementVersion || !privacyPolicyVersion) {
    throw new Error('协议版本加载失败，请稍后重试')
  }

  return {
    user_agreement_version: userAgreementVersion,
    privacy_policy_version: privacyPolicyVersion,
    consented_at: new Date().toISOString()
  }
}
