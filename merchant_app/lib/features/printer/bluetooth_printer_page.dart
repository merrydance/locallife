import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';
import 'package:merchant_app/widgets/merchant_secondary_button.dart';
import 'package:merchant_app/widgets/merchant_status_badge.dart';

class BluetoothPrinterPage extends ConsumerStatefulWidget {
  const BluetoothPrinterPage({super.key});

  @override
  ConsumerState<BluetoothPrinterPage> createState() =>
      _BluetoothPrinterPageState();
}

class _BluetoothPrinterPageState extends ConsumerState<BluetoothPrinterPage> {
  @override
  void initState() {
    super.initState();
    // Start scan on init if not already connected
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (ref.read(printerProvider).connectedDevice == null) {
        ref.read(printerProvider.notifier).startScan();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(printerProvider);
    final notifier = ref.read(printerProvider.notifier);
    final availableDevices = state.scanResults
        .where((result) => result.device.platformName.isNotEmpty)
        .toList();

    return Scaffold(
      appBar: AppBar(
        title: const Text('蓝牙打印机'),
        actions: [
          if (state.isScanning)
            Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16.0),
                child: SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
              ),
            )
          else
            IconButton(
              icon: const Icon(Icons.refresh),
              onPressed: () => notifier.startScan(),
            ),
        ],
      ),
      body: ListView(
        padding: EdgeInsets.zero,
        children: [
          MerchantContentShell(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (state.error != null) ...[
                  Container(
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    decoration: BoxDecoration(
                      color: AppColors.dangerSoft,
                      borderRadius: BorderRadius.circular(AppRadius.xl),
                    ),
                    child: Text(
                      state.error!,
                      textAlign: TextAlign.center,
                      style: const TextStyle(
                        color: AppColors.tertiary,
                        fontWeight: FontWeight.w600,
                        height: 1.45,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                ],
                _buildPageSummary(context, state, notifier),
                const SizedBox(height: AppSpacing.lg),
                if (state.connectedDevice != null) ...[
                  _buildSectionTitle('当前已连接设备'),
                  const SizedBox(height: AppSpacing.sm),
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(AppSpacing.xl),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Container(
                                width: 44,
                                height: 44,
                                decoration: BoxDecoration(
                                  color: AppColors.surfaceLow,
                                  borderRadius: BorderRadius.circular(
                                    AppRadius.lg,
                                  ),
                                ),
                                child: const Icon(
                                  Icons.print_outlined,
                                  color: AppColors.primary,
                                ),
                              ),
                              const SizedBox(width: AppSpacing.md),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      state.connectedDevice!.name.isNotEmpty
                                          ? state.connectedDevice!.name
                                          : '未知蓝牙打印机',
                                      style: const TextStyle(
                                        fontSize: 16,
                                        fontWeight: FontWeight.w700,
                                      ),
                                    ),
                                    const SizedBox(height: AppSpacing.xs),
                                    Text(
                                      state.connectedDevice!.id,
                                      style: const TextStyle(
                                        color: AppColors.onSurfaceVariant,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const MerchantStatusBadge(
                                label: '已连接',
                                tone: MerchantStatusTone.positive,
                              ),
                            ],
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          MerchantSecondaryButton(
                            expand: true,
                            label: '断开当前打印机',
                            onPressed: () => notifier.disconnect(),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                ],
                _buildSectionTitle('可用蓝牙设备'),
                const SizedBox(height: AppSpacing.sm),
                if (availableDevices.isEmpty && !state.isScanning)
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(AppSpacing.xxl),
                      child: Column(
                        children: [
                          Container(
                            width: 72,
                            height: 72,
                            decoration: BoxDecoration(
                              color: AppColors.surfaceLow,
                              borderRadius: BorderRadius.circular(AppRadius.xl),
                            ),
                            child: const Icon(
                              Icons.bluetooth_disabled_rounded,
                              color: AppColors.onSurfaceVariant,
                              size: 34,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          const Text(
                            '暂未发现可连接设备',
                            style: TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          const Text(
                            '请确认打印机已开机、蓝牙已开启，并保持设备在附近。',
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              color: AppColors.onSurfaceVariant,
                              height: 1.45,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          MerchantPrimaryButton(
                            label: state.isScanning ? '正在扫描...' : '重新扫描设备',
                            onPressed: state.isScanning
                                ? null
                                : () => notifier.startScan(),
                          ),
                        ],
                      ),
                    ),
                  )
                else
                  ...availableDevices.map(
                    (result) => Padding(
                      padding: const EdgeInsets.only(bottom: AppSpacing.md),
                      child: _PrinterDeviceCard(
                        name: result.device.platformName,
                        deviceId: result.device.remoteId.str,
                        isConnected:
                            state.connectedDevice?.id ==
                            result.device.remoteId.str,
                        isConnecting:
                            state.isConnecting &&
                            state.connectedDevice?.id ==
                                result.device.remoteId.str,
                        onConnect: () =>
                            notifier.connectToDevice(result.device),
                      ),
                    ),
                  ),
                const SizedBox(height: AppSpacing.xl),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPageSummary(
    BuildContext context,
    PrinterState state,
    PrinterNotifier notifier,
  ) {
    final statusBadge = state.connectedDevice != null
        ? const MerchantStatusBadge(
            label: '打印可用',
            tone: MerchantStatusTone.positive,
          )
        : MerchantStatusBadge(
            label: state.isScanning ? '扫描中' : '待连接',
            tone: state.isScanning
                ? MerchantStatusTone.warning
                : MerchantStatusTone.neutral,
          );

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        '打印设备管理',
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      const Text(
                        '连接蓝牙小票打印机后，可用于订单小票和后续自动打印能力。',
                        style: TextStyle(
                          color: AppColors.onSurfaceVariant,
                          height: 1.45,
                        ),
                      ),
                    ],
                  ),
                ),
                statusBadge,
              ],
            ),
            const SizedBox(height: AppSpacing.lg),
            MerchantPrimaryButton(
              label: state.isScanning ? '正在扫描设备...' : '扫描附近设备',
              onPressed: state.isScanning ? null : () => notifier.startScan(),
              icon: const Icon(Icons.bluetooth_searching_rounded),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSectionTitle(String title) {
    return Text(
      title,
      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
    );
  }
}

class _PrinterDeviceCard extends StatelessWidget {
  const _PrinterDeviceCard({
    required this.name,
    required this.deviceId,
    required this.isConnected,
    required this.isConnecting,
    required this.onConnect,
  });

  final String name;
  final String deviceId;
  final bool isConnected;
  final bool isConnecting;
  final VoidCallback onConnect;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: AppColors.surfaceLow,
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
              child: const Icon(
                Icons.bluetooth_audio_outlined,
                color: AppColors.primary,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    deviceId,
                    style: const TextStyle(color: AppColors.onSurfaceVariant),
                  ),
                  const SizedBox(height: AppSpacing.md),
                  if (isConnected)
                    const MerchantStatusBadge(
                      label: '当前连接',
                      tone: MerchantStatusTone.positive,
                    )
                  else if (isConnecting)
                    const MerchantStatusBadge(
                      label: '连接中',
                      tone: MerchantStatusTone.warning,
                    )
                  else
                    Align(
                      alignment: Alignment.centerLeft,
                      child: OutlinedButton(
                        onPressed: onConnect,
                        child: const Text('连接设备'),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
