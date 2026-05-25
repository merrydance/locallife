import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_detail_provider.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/ui/widgets/table_config_sheet.dart';
import 'package:merchant_app/features/table/ui/widgets/table_image_gallery.dart';
import 'package:merchant_app/features/table/ui/widgets/table_qrcode_sheet.dart';

class TableDetailScreen extends ConsumerStatefulWidget {
  final int tableId;

  const TableDetailScreen({super.key, required this.tableId});

  @override
  ConsumerState<TableDetailScreen> createState() => _TableDetailScreenState();
}

class _TableDetailScreenState extends ConsumerState<TableDetailScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(tableDetailProvider.notifier).loadDetail(widget.tableId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final detailState = ref.watch(tableDetailProvider);
    final table = detailState.table;

    return Scaffold(
      appBar: AppBar(
        title: Text(table != null ? '桌台 ${table.tableNo}' : '桌台详情'),
        actions: [
          if (table != null)
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: () => _showEditSheet(context, table),
            ),
        ],
      ),
      body: _buildBody(detailState),
    );
  }

  Widget _buildBody(TableDetailState detailState) {
    if (detailState.isLoading && detailState.table == null) {
      return const Center(child: CircularProgressIndicator());
    }

    if (detailState.error != null && detailState.table == null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              detailState.error!,
              style: const TextStyle(color: AppColors.danger),
            ),
            const SizedBox(height: AppSpacing.lg),
            ElevatedButton(
              onPressed: () => ref
                  .read(tableDetailProvider.notifier)
                  .loadDetail(widget.tableId),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    final table = detailState.table!;

    return RefreshIndicator(
      onRefresh: () =>
          ref.read(tableDetailProvider.notifier).loadDetail(widget.tableId),
      child: ListView(
        padding: const EdgeInsets.all(AppSpacing.lg),
        children: [
          // 图片区域
          TableImageGallery(
            tableId: widget.tableId,
            images: detailState.images,
          ),
          const SizedBox(height: AppSpacing.lg),

          // 基础信息卡片
          _buildInfoCard(table),
          const SizedBox(height: AppSpacing.md),

          // 标签区域
          if (detailState.tags.isNotEmpty) ...[
            _buildTagsSection(detailState.tags),
            const SizedBox(height: AppSpacing.md),
          ],

          // 预订信息
          if (table.currentReservation != null) ...[
            _buildReservationCard(table.currentReservation!),
            const SizedBox(height: AppSpacing.md),
          ],

          // 操作按钮区域
          _buildActionButtons(table),
        ],
      ),
    );
  }

  // ==================== 基础信息卡片 ====================

  Widget _buildInfoCard(TableModel table) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    table.tableNo,
                    style: const TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                _buildStatusBadge(table.status),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            _buildInfoRow(Icons.chair_alt, '类型', table.tableType.label),
            _buildInfoRow(Icons.people_outline, '容量', '${table.capacity} 人'),
            if (table.minimumSpend != null)
              _buildInfoRow(
                Icons.payments_outlined,
                '最低消费',
                '¥${(table.minimumSpend! / 100).toStringAsFixed(0)}',
              ),
            if (table.description != null && table.description!.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.md),
              const Text(
                '描述',
                style: TextStyle(
                  fontSize: 12,
                  color: AppColors.onSurfaceVariant,
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(
                table.description!,
                style: const TextStyle(fontSize: 14, height: 1.5),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildInfoRow(IconData icon, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Icon(icon, size: 18, color: AppColors.onSurfaceVariant),
          const SizedBox(width: AppSpacing.sm),
          Text(
            label,
            style: const TextStyle(
              fontSize: 14,
              color: AppColors.onSurfaceVariant,
            ),
          ),
          const Spacer(),
          Text(
            value,
            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusBadge(TableStatus status) {
    Color bgColor;
    Color textColor;
    switch (status) {
      case TableStatus.available:
        bgColor = AppColors.positiveSoft;
        textColor = AppColors.positive;
      case TableStatus.occupied:
        bgColor = AppColors.warningSoft;
        textColor = AppColors.warning;
      case TableStatus.disabled:
        bgColor = AppColors.surfaceLow;
        textColor = AppColors.onSurfaceVariant;
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
      child: Text(
        status.label,
        style: TextStyle(
          color: textColor,
          fontWeight: FontWeight.w600,
          fontSize: 12,
        ),
      ),
    );
  }

  // ==================== 标签区域 ====================

  Widget _buildTagsSection(List<TableTagInfo> tags) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              '标签',
              style: TextStyle(fontSize: 14, fontWeight: FontWeight.w600),
            ),
            const SizedBox(height: AppSpacing.sm),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: tags
                  .map(
                    (tag) => Chip(
                      label: Text(
                        tag.name,
                        style: const TextStyle(fontSize: 12),
                      ),
                      backgroundColor: AppColors.positiveSoft,
                      side: BorderSide.none,
                      padding: EdgeInsets.zero,
                      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      visualDensity: VisualDensity.compact,
                    ),
                  )
                  .toList(),
            ),
          ],
        ),
      ),
    );
  }

  // ==================== 预订信息 ====================

  Widget _buildReservationCard(ReservationInfo reservation) {
    return Card(
      color: AppColors.warningSoft,
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(
                  Icons.event_available,
                  size: 18,
                  color: AppColors.warning,
                ),
                const SizedBox(width: AppSpacing.sm),
                const Text(
                  '当前预订',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: AppColors.warning,
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            _buildReservationRow('联系人', reservation.contactName),
            _buildReservationRow('电话', reservation.contactPhone),
            _buildReservationRow('人数', '${reservation.guestCount} 人'),
            _buildReservationRow('时间', reservation.reservationTime),
            if (reservation.notes != null && reservation.notes!.isNotEmpty)
              _buildReservationRow('备注', reservation.notes!),
          ],
        ),
      ),
    );
  }

  Widget _buildReservationRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        children: [
          SizedBox(
            width: 56,
            child: Text(
              label,
              style: const TextStyle(
                fontSize: 13,
                color: AppColors.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(child: Text(value, style: const TextStyle(fontSize: 13))),
        ],
      ),
    );
  }

  // ==================== 操作按钮 ====================

  Widget _buildActionButtons(TableModel table) {
    return Column(
      children: [
        // 二维码
        SizedBox(
          width: double.infinity,
          child: OutlinedButton.icon(
            onPressed: () => _showQRCodeSheet(context, table),
            icon: const Icon(Icons.qr_code_2),
            label: const Text('桌台二维码'),
            style: OutlinedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
            ),
          ),
        ),
        const SizedBox(height: AppSpacing.md),

        // 状态切换
        if (table.status == TableStatus.available)
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () async {
                final success = await ref
                    .read(tableProvider.notifier)
                    .updateTableStatus(table.id, 'occupied');
                if (success && mounted) {
                  ref
                      .read(tableDetailProvider.notifier)
                      .loadDetail(widget.tableId);
                }
              },
              child: const Text('开台'),
            ),
          )
        else if (table.status == TableStatus.occupied)
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () async {
                final success = await ref
                    .read(tableProvider.notifier)
                    .updateTableStatus(table.id, 'available');
                if (success && mounted) {
                  ref
                      .read(tableDetailProvider.notifier)
                      .loadDetail(widget.tableId);
                }
              },
              child: const Text('清台'),
            ),
          ),
      ],
    );
  }

  // ==================== 操作方法 ====================

  void _showEditSheet(BuildContext context, TableModel table) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => TableConfigSheet(existingTable: table),
    ).then((_) {
      // 编辑完成后刷新详情
      ref.read(tableDetailProvider.notifier).loadDetail(widget.tableId);
      ref.read(tableProvider.notifier).fetchTables();
    });
  }

  void _showQRCodeSheet(BuildContext context, TableModel table) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) =>
          TableQRCodeSheet(tableId: table.id, tableNo: table.tableNo),
    );
  }
}
