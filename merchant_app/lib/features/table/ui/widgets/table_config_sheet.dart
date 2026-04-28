import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

class TableConfigSheet extends ConsumerStatefulWidget {
  final TableModel? existingTable;

  const TableConfigSheet({super.key, this.existingTable});

  @override
  ConsumerState<TableConfigSheet> createState() => _TableConfigSheetState();
}

class _TableConfigSheetState extends ConsumerState<TableConfigSheet> {
  final _formKey = GlobalKey<FormState>();
  late TextEditingController _tableNoController;
  late TextEditingController _capacityController;
  late TableType _selectedType;
  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    _tableNoController = TextEditingController(text: widget.existingTable?.tableNo ?? '');
    _capacityController = TextEditingController(text: widget.existingTable?.capacity.toString() ?? '4');
    _selectedType = widget.existingTable?.tableType ?? TableType.table;
  }

  @override
  void dispose() {
    _tableNoController.dispose();
    _capacityController.dispose();
    super.dispose();
  }

  Future<void> _submit(BuildContext context) async {
    if (!_formKey.currentState!.validate()) return;

    setState(() => _isSubmitting = true);

    final tableNo = _tableNoController.text.trim();
    final capacity = int.tryParse(_capacityController.text.trim()) ?? 4;

    bool success = false;
    if (widget.existingTable == null) {
      success = await ref.read(tableProvider.notifier).createTable(
        tableNo: tableNo,
        tableType: _selectedType.value,
        capacity: capacity,
      );
    } else {
      success = await ref.read(tableProvider.notifier).updateTable(
        tableId: widget.existingTable!.id,
        tableNo: tableNo,
        tableType: _selectedType.value,
        capacity: capacity,
      );
    }

    setState(() => _isSubmitting = false);
    if (!context.mounted) return;

    if (success) {
      Navigator.pop(context);
    } else {
      final error = ref.read(tableProvider).error ?? '保存失败，请重试';
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final isEditing = widget.existingTable != null;

    return Padding(
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.xl),
        decoration: const BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xxl)),
        ),
        child: SafeArea(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      isEditing ? '编辑桌台' : '新增桌台',
                      style: const TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close),
                      onPressed: () => Navigator.pop(context),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),

                // Table Type Selection
                Row(
                  children: [
                    Expanded(
                      child: _buildTypeRadio(TableType.table, Icons.chair_alt),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: _buildTypeRadio(TableType.room, Icons.meeting_room),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),

                // Table No Input
                TextFormField(
                  controller: _tableNoController,
                  decoration: const InputDecoration(
                    labelText: '桌台编号 (如 A01, V888)',
                    border: OutlineInputBorder(),
                  ),
                  validator: (value) {
                    if (value == null || value.trim().isEmpty) {
                      return '请输入桌台编号';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: AppSpacing.lg),

                // Capacity Input
                TextFormField(
                  controller: _capacityController,
                  keyboardType: TextInputType.number,
                  decoration: const InputDecoration(
                    labelText: '就餐人数容量',
                    border: OutlineInputBorder(),
                  ),
                  validator: (value) {
                    if (value == null || value.trim().isEmpty) {
                      return '请输入人数容量';
                    }
                    if (int.tryParse(value.trim()) == null) {
                      return '请输入有效的数字';
                    }
                    return null;
                  },
                ),
                const SizedBox(height: AppSpacing.xxl),

                MerchantPrimaryButton(
                  label: isEditing ? '保存修改' : '确认新增',
                  expand: true,
                  isLoading: _isSubmitting,
                  onPressed: () => _submit(context),
                ),

                if (isEditing) ...[
                  const SizedBox(height: AppSpacing.md),
                  SizedBox(
                    width: double.infinity,
                    height: 48,
                    child: TextButton(
                      onPressed: () async {
                        final confirm = await showDialog<bool>(
                          context: context,
                          builder: (context) => AlertDialog(
                            title: const Text('删除桌台'),
                            content: const Text('确定要删除此桌台吗？如果有进行中的订单将无法删除。'),
                            actions: [
                              TextButton(
                                onPressed: () => Navigator.pop(context, false),
                                child: const Text('取消'),
                              ),
                              TextButton(
                                onPressed: () => Navigator.pop(context, true),
                                style: TextButton.styleFrom(foregroundColor: AppColors.danger),
                                child: const Text('确定删除'),
                              ),
                            ],
                          ),
                        );

                        if (confirm == true && context.mounted) {
                          setState(() => _isSubmitting = true);
                          final success = await ref.read(tableProvider.notifier).deleteTable(widget.existingTable!.id);
                          if (!context.mounted) return;
                          setState(() => _isSubmitting = false);

                          if (success) {
                            Navigator.pop(context);
                          } else {
                            final error = ref.read(tableProvider).error ?? '删除失败';
                            ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
                          }
                        }
                      },
                      style: TextButton.styleFrom(
                        foregroundColor: AppColors.danger,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppRadius.lg),
                        ),
                      ),
                      child: const Text('删除桌台'),
                    ),
                  ),
                ]
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTypeRadio(TableType type, IconData icon) {
    final isSelected = _selectedType == type;
    return InkWell(
      onTap: () => setState(() => _selectedType = type),
      borderRadius: BorderRadius.circular(AppRadius.lg),
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.primary.withValues(alpha: 0.1) : AppColors.surfaceLow,
          border: Border.all(
            color: isSelected ? AppColors.primary : Colors.transparent,
            width: 2,
          ),
          borderRadius: BorderRadius.circular(AppRadius.lg),
        ),
        child: Column(
          children: [
            Icon(icon, color: isSelected ? AppColors.primary : AppColors.onSurfaceVariant),
            const SizedBox(height: 4),
            Text(
              type.label,
              style: TextStyle(
                color: isSelected ? AppColors.primary : AppColors.onSurfaceVariant,
                fontWeight: isSelected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
