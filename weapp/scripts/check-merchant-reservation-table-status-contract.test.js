const assert = require('assert')
const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const vm = require('vm')

const ROOT = path.resolve(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(ROOT, relativePath), 'utf8')
}

function loadTsModule(relativePath, requireStub = () => ({})) {
  const sourcePath = path.join(ROOT, relativePath)
  const source = fs.readFileSync(sourcePath, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2018,
      esModuleInterop: true
    }
  }).outputText

  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: requireStub,
    console
  }

  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled, sandbox, { filename: sourcePath })
  return sandbox.module.exports
}

const reservationView = loadTsModule('miniprogram/pages/merchant/_utils/merchant-reservations-view.ts', (id) => {
  if (id === '../_main_shared/miniprogram_npm/dayjs/index') {
    return () => ({
      isValid: () => true,
      format: () => '12:00'
    })
  }
  if (id === '../_main_shared/api/reservation') {
    return {
      formatReservationStatus: (status) => ({ confirmed: '已确认', checked_in: '已到店' }[status] || status),
      getReservationStatusTheme: () => 'primary',
      isReservationPendingPayment: (status) => status === 'pending',
      getMerchantReservationActionState: () => ({
        canEdit: true,
        canCancel: true,
        canConfirm: true,
        canCheckIn: false,
        canStartCooking: false,
        canNoShow: false,
        canComplete: false,
        primaryActionKey: 'confirm',
        primaryActionLabel: '确认预订',
        showMoreActions: false
      })
    }
  }
  throw new Error(`unexpected require: ${id}`)
})

const confirmDialog = reservationView.getReservationActionDialogConfig('confirm', '王先生')
assert(
  confirmDialog.content.includes('不会占用桌台') &&
    confirmDialog.content.includes('开台') &&
    confirmDialog.content.includes('就餐中'),
  'confirm dialog must state that confirming a reservation does not occupy a table; opening a dining session does'
)

const completeDialog = reservationView.getReservationActionDialogConfig('complete', '王先生')
assert(
  completeDialog.content.includes('若当前桌台仍关联该预订') &&
    completeDialog.content.includes('释放回空闲'),
  'complete dialog must scope table release to the table still being linked to the reservation'
)

const tablePage = read('miniprogram/pages/merchant/tables/index.ts')
assert(
  tablePage.includes('会关闭当前就餐会话') &&
    tablePage.includes('如有关联预订') &&
    tablePage.includes('一并完成') &&
    tablePage.includes("updateTableStatus(id, { status: 'available' })"),
  'manual table release copy must disclose active dining-session close and linked reservation completion before sending available status'
)

console.log('check-merchant-reservation-table-status-contract: reservation/table status copy is aligned')
