import 'package:flutter/material.dart';
import 'package:app_settings/app_settings.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

class PermissionGuidePage extends StatelessWidget {
  const PermissionGuidePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('自启动与保活设置'),
      ),
      body: SingleChildScrollView(
        child: MerchantContentShell(
          child: Column(
            children: [
              Container(
                padding: const EdgeInsets.all(AppSpacing.xl),
                decoration: BoxDecoration(
                  color: AppColors.warningSoft,
                  borderRadius: BorderRadius.circular(AppRadius.xl),
                ),
                child: const Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      Icons.warning_amber_rounded,
                      color: AppColors.secondary,
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Text(
                        '为保证息屏或退到后台后仍能收到订单提醒，请按手机品牌完成以下保活设置。',
                        style: TextStyle(
                          color: AppColors.onSurface,
                          height: 1.5,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              _buildBrandGuide(
                brand: '华为 / 荣耀 (EMUI / HarmonyOS)',
                steps: [
                  '1. 在应用启动管理中关闭自动管理，并手动开启自启动、关联启动、后台活动。',
                  '2. 在电池设置中开启休眠时保持网络连接。',
                  '3. 将乐客来福加入不受电池优化限制名单。',
                ],
              ),
              _buildBrandGuide(
                brand: '小米 / Redmi (MIUI / HyperOS)',
                steps: [
                  '1. 在应用管理中开启乐客来福自启动。',
                  '2. 在省电与电池设置中改为无限制。',
                  '3. 在最近任务界面锁定乐客来福卡片，避免被系统回收。',
                ],
              ),
              _buildBrandGuide(
                brand: 'OPPO / Realme (ColorOS)',
                steps: [
                  '1. 在耗电保护中开启允许后台运行。',
                  '2. 在自启动管理中允许乐客来福自动启动。',
                  '3. 允许通知与状态栏提醒，避免新订单无提示。',
                ],
              ),
              _buildBrandGuide(
                brand: 'vivo (OriginOS / Funtouch)',
                steps: [
                  '1. 在后台耗电管理中允许后台高耗电。',
                  '2. 在权限管理中开启乐客来福自启动。',
                ],
              ),
              const SizedBox(height: AppSpacing.xl),
              MerchantPrimaryButton(
                label: '前往系统设置',
                onPressed: () {
                  AppSettings.openAppSettings();
                },
                icon: const Icon(Icons.settings),
              ),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBrandGuide({required String brand, required List<String> steps}) {
    return Card(
      margin: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
      child: ExpansionTile(
        title: Text(
          brand,
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
        leading: const Icon(Icons.phone_android, color: AppColors.primary),
        children: [
          Padding(
            padding: const EdgeInsets.only(
              left: AppSpacing.lg,
              right: AppSpacing.lg,
              bottom: AppSpacing.lg,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: steps
                  .map((step) => Padding(
                        padding: const EdgeInsets.only(top: AppSpacing.sm),
                        child: Text(
                          step,
                          style: const TextStyle(fontSize: 14, height: 1.5),
                        ),
                      ))
                  .toList(),
            ),
          ),
        ],
      ),
    );
  }
}
