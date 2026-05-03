import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_status_badge.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isAuthenticated = ref.watch(
      authProvider.select((state) => state.isAuthenticated),
    );
    final printerState = ref.watch(printerProvider);
    final notificationSettings = ref.watch(notificationSettingsProvider);
    final notificationSettingsNotifier = ref.read(
      notificationSettingsProvider.notifier,
    );

    return Scaffold(
      appBar: AppBar(title: const Text('系统设置')),
      body: ListView(
        padding: EdgeInsets.zero,
        children: [
          MerchantContentShell(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _buildSectionCard(
                  title: '硬件与连接',
                  description: '查看当前打印与消息接收环境是否可用。',
                  children: [
                    _buildActionTile(
                      context: context,
                      icon: Icons.print_outlined,
                      title: '小票打印机',
                      subtitle: printerState.connectedDevice != null
                          ? '已连接 ${printerState.connectedDevice!.platformName}'
                          : '未连接蓝牙打印机',
                      badge: MerchantStatusBadge(
                        label: printerState.connectedDevice != null
                            ? '已连接'
                            : '未连接',
                        tone: printerState.connectedDevice != null
                            ? MerchantStatusTone.positive
                            : MerchantStatusTone.neutral,
                      ),
                      onTap: () => context.push('/printer'),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _buildSectionCard(
                  title: '提醒与保活',
                  description: '设置来单提醒、接单方式和接单后打印，并检查系统保活状态。',
                  children: [
                    _buildToggleTile(
                      icon: Icons.music_note_outlined,
                      title: '铃声提醒',
                      subtitle: '有新订单时播放提示音。',
                      value: notificationSettings.soundEnabled,
                      onChanged: notificationSettingsNotifier.setSoundEnabled,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    _buildToggleTile(
                      icon: Icons.record_voice_over_outlined,
                      title: '语音播报',
                      subtitle: '有新订单时播报订单号和金额。',
                      value: notificationSettings.voiceEnabled,
                      onChanged: notificationSettingsNotifier.setVoiceEnabled,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    _buildToggleTile(
                      icon: Icons.flash_auto_outlined,
                      title: '自动接单',
                      subtitle: '新订单到达后自动确认接单；如果失败，订单仍会保留为待处理。',
                      value: notificationSettings.autoAcceptEnabled,
                      onChanged:
                          notificationSettingsNotifier.setAutoAcceptEnabled,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    _buildToggleTile(
                      icon: Icons.print_outlined,
                      title: '接单后自动打印',
                      subtitle: '手工接单或自动接单成功后，如已连接蓝牙打印机则自动打印小票。',
                      value: notificationSettings.autoPrintAfterAcceptEnabled,
                      onChanged: notificationSettingsNotifier
                          .setAutoPrintAfterAcceptEnabled,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    _buildActionTile(
                      context: context,
                      icon: Icons.shutter_speed_outlined,
                      title: '自启动与保活设置',
                      subtitle: '如果后台偶尔漏提醒，请按品牌指引完成系统设置。',
                      badge: const MerchantStatusBadge(
                        label: '需检查',
                        tone: MerchantStatusTone.warning,
                      ),
                      onTap: () => context.push('/settings/permission-guide'),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                _buildSectionCard(
                  title: '账号与应用',
                  description: isAuthenticated
                      ? '查看版本、帮助信息，或退出当前商户账号。'
                      : '查看版本、帮助信息，或先完成商户绑定后开始接单。',
                  children: [
                    _buildActionTile(
                      context: context,
                      icon: Icons.info_outline,
                      title: '关于乐客来福',
                      subtitle: '查看版本信息、检查更新和协议入口。',
                      onTap: () => context.push('/settings/about'),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    if (isAuthenticated)
                      _buildActionTile(
                        context: context,
                        icon: Icons.logout,
                        iconColor: AppColors.tertiary,
                        title: '退出登录',
                        subtitle: '退出后需要重新使用绑定码登录。',
                        titleColor: AppColors.tertiary,
                        onTap: () async {
                          final confirm = await showDialog<bool>(
                            context: context,
                            builder: (context) => AlertDialog(
                              title: const Text('退出登录'),
                              content: const Text('确定要退出当前商户账号吗？'),
                              actions: [
                                TextButton(
                                  onPressed: () =>
                                      Navigator.pop(context, false),
                                  child: const Text('取消'),
                                ),
                                TextButton(
                                  onPressed: () => Navigator.pop(context, true),
                                  child: const Text('确定退出'),
                                ),
                              ],
                            ),
                          );
                          if (confirm == true) {
                            ref
                                .read(workingStatusProvider.notifier)
                                .setStatus(false);
                            ref.read(orderProvider.notifier).clearOrders();
                            ref.read(authProvider.notifier).logout();
                          }
                        },
                      )
                    else
                      _buildActionTile(
                        context: context,
                        icon: Icons.login,
                        title: '绑定商户',
                        subtitle: '输入微信小程序生成的 6 位绑定码后开始接单。',
                        onTap: () => context.go('/login'),
                      ),
                  ],
                ),
                const SizedBox(height: AppSpacing.xl),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildToggleTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required bool value,
    required ValueChanged<bool> onChanged,
  }) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surfaceLow,
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: AppColors.primary),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  subtitle,
                  style: const TextStyle(
                    color: AppColors.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Switch(
            value: value,
            onChanged: onChanged,
            activeThumbColor: AppColors.surfaceLowest,
            activeTrackColor: AppColors.primaryContainer,
          ),
        ],
      ),
    );
  }

  Widget _buildSectionCard({
    required String title,
    required String description,
    required List<Widget> children,
  }) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
            ),
            const SizedBox(height: AppSpacing.xs),
            Text(
              description,
              style: const TextStyle(
                color: AppColors.onSurfaceVariant,
                height: 1.45,
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            ...children,
          ],
        ),
      ),
    );
  }

  Widget _buildActionTile({
    required BuildContext context,
    required IconData icon,
    required String title,
    required String subtitle,
    VoidCallback? onTap,
    Widget? badge,
    Color? iconColor,
    Color? titleColor,
  }) {
    final content = Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surfaceLow,
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, color: iconColor ?? AppColors.primary),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: titleColor ?? AppColors.onSurface,
                  ),
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  subtitle,
                  style: const TextStyle(
                    color: AppColors.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          badge ??
              const Icon(
                Icons.chevron_right,
                color: AppColors.onSurfaceVariant,
              ),
        ],
      ),
    );

    if (onTap == null) {
      return content;
    }

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppRadius.lg),
      child: content,
    );
  }
}
