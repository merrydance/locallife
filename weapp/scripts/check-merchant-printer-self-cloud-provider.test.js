const assert = require('assert')
const fs = require('fs')
const path = require('path')

const repoRoot = path.join(__dirname, '..')

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8')
}

const apiSource = read('miniprogram/api/table-device-management.ts')
const editTsSource = read('miniprogram/pages/merchant/printers/edit/index.ts')
const editWxmlSource = read('miniprogram/pages/merchant/printers/edit/index.wxml')
const reconciliationViewSource = read('miniprogram/utils/printer-reconciliation-view.ts')
const orderDetailViewSource = read('miniprogram/pages/merchant/_utils/merchant-order-detail-view.ts')
const printAnomaliesSource = read('miniprogram/pages/merchant/orders/print-anomalies/index.ts')

assert(
  apiSource.includes("'self_cloud'"),
  'printer API contract should include backend self_cloud provider type'
)

assert(
  editTsSource.includes("value: 'self_cloud'") &&
    editTsSource.includes("label: '东为打印机'"),
  'printer edit page should offer Dongwei printers beside Feie, Shangpeng, and Yilianyun'
)

assert(
  editTsSource.includes("type DirectCreatePrinterType = 'feieyun' | 'shangpeng' | 'self_cloud'") &&
    /formData\.printer_type !== 'feieyun' &&\s*formData\.printer_type !== 'shangpeng' &&\s*formData\.printer_type !== 'self_cloud'/.test(editTsSource),
  'Dongwei/self_cloud printers should use the direct create-printer flow, not the Yilianyun authorization flow'
)

assert(
  editTsSource.includes("self_cloud: '东为打印机'") &&
    editTsSource.includes('SN 和绑定码') &&
    editTsSource.includes('设备贴纸上的短码') &&
    editWxmlSource.includes('{{selectedPrinterTypeLabel}}') &&
    editWxmlSource.includes('{{printerSnLabel}}') &&
    editWxmlSource.includes('{{printerKeyLabel}}'),
  'printer edit view should render Dongwei label and merchant-facing SN/binding-code copy'
)

for (const [source, name] of [
  [reconciliationViewSource, 'printer reconciliation view'],
  [orderDetailViewSource, 'merchant order print-job view'],
  [printAnomaliesSource, 'merchant print anomalies view']
]) {
  assert(
    source.includes("self_cloud: '东为打印机'"),
    `${name} should label self_cloud printer records as Dongwei printers`
  )
}

console.log('check-merchant-printer-self-cloud-provider: Dongwei/self_cloud provider is selectable, directly creatable, and labeled across merchant print views')
