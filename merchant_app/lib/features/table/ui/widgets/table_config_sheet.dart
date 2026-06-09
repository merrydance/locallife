import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_image_provider.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/ui/widgets/table_tag_selector.dart';
import 'package:merchant_app/widgets/merchant_image_picker.dart';
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
  late TextEditingController _descriptionController;
  late TextEditingController _minimumSpendController;
  late TextEditingController _accessCodeController;
  late TableType _selectedType;
  late List<int> _selectedTagIds;
  bool _isSubmitting = false;
  int? _uploadedMediaAssetId;
  String? _uploadedImageUrl;

  @override
  void initState() {
    super.initState();
    _tableNoController = TextEditingController(
      text: widget.existingTable?.tableNo ?? '',
    );
    _capacityController = TextEditingController(
      text: widget.existingTable?.capacity.toString() ?? '4',
    );
    _descriptionController = TextEditingController(
      text: widget.existingTable?.description ?? '',
    );
    _minimumSpendController = TextEditingController(
      text: widget.existingTable?.minimumSpend != null
          ? (widget.existingTable!.minimumSpend! / 100).toStringAsFixed(0)
          : '',
    );
    _accessCodeController = TextEditingController();
    _selectedType = widget.existingTable?.tableType ?? TableType.table;
    _selectedTagIds =
        widget.existingTable?.tags.map((t) => t.id).toList() ?? [];
  }

  @override
  void dispose() {
    _tableNoController.dispose();
    _capacityController.dispose();
    _descriptionController.dispose();
    _minimumSpendController.dispose();
    _accessCodeController.dispose();
    super.dispose();
  }

  Future<void> _submit(BuildContext context) async {
    if (!_formKey.currentState!.validate()) return;

    setState(() => _isSubmitting = true);

    final tableNo = _tableNoController.text.trim();
    final capacity = int.tryParse(_capacityController.text.trim()) ?? 4;
    final description = _descriptionController.text.trim();
    final minimumSpendText = _minimumSpendController.text.trim();
    final minimumSpend = minimumSpendText.isNotEmpty
        ? ((double.tryParse(minimumSpendText) ?? 0) * 100).round()
        : null;
    final accessCode = _accessCodeController.text.trim();

    bool success = false;
    int? createdTableId;
    if (widget.existingTable == null) {
      final result = await ref
          .read(tableProvider.notifier)
          .createTableAndReturnId(
            tableNo: tableNo,
            tableType: _selectedType.value,
            capacity: capacity,
            description: description.isNotEmpty ? description : null,
            minimumSpend: minimumSpend,
            accessCode: accessCode.isNotEmpty ? accessCode : null,
            tagIds: _selectedTagIds,
          );
      success = result != null;
      createdTableId = result;
    } else {
      success = await ref
          .read(tableProvider.notifier)
          .updateTable(
            tableId: widget.existingTable!.id,
            tableNo: tableNo,
            tableType: _selectedType.value,
            capacity: capacity,
            description: description.isNotEmpty ? description : null,
            minimumSpend: minimumSpend,
            accessCode: accessCode.isNotEmpty ? accessCode : null,
            tagIds: _selectedTagIds,
          );
    }

    // 新建成功且有已上传的图片，则关联图片到桌台
    if (success && _uploadedMediaAssetId != null) {
      final targetTableId = createdTableId ?? widget.existingTable?.id;
      if (targetTableId != null) {
        await ref
            .read(tableImageProvider.notifier)
            .addImage(targetTableId, _uploadedMediaAssetId!);
      }
    }

    // 刷新桌台列表确保数据同步
    if (success) {
      await ref.read(tableProvider.notifier).fetchTables();
    }

    setState(() => _isSubmitting = false);
    if (!context.mounted) return;

    if (success) {
      Navigator.pop(context);
    } else {
      final error = ref.read(tableProvider).error ?? '保存失败，请重试';
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(error)));
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
          borderRadius: BorderRadius.vertical(
            top: Radius.circular(AppRadius.xxl),
          ),
        ),
        child: SafeArea(
          child: ConstrainedBox(
            constraints: BoxConstraints(
              maxHeight: MediaQuery.of(context).size.height * 0.85,
            ),
            child: Form(
              key: _formKey,
              child: SingleChildScrollView(
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
                          child: _buildTypeRadio(
                            TableType.table,
                            Icons.chair_alt,
                          ),
                        ),
                        const SizedBox(width: AppSpacing.md),
                        Expanded(
                          child: _buildTypeRadio(
                            TableType.room,
                            Icons.meeting_room,
                          ),
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
                    const SizedBox(height: AppSpacing.lg),

                    // Description Input
                    TextFormField(
                      controller: _descriptionController,
                      maxLines: 3,
                      minLines: 1,
                      decoration: const InputDecoration(
                        labelText: '描述 (选填)',
                        hintText: '如：靠窗雅座，环境安静',
                        border: OutlineInputBorder(),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.lg),

                    // Minimum Spend Input
                    TextFormField(
                      controller: _minimumSpendController,
                      keyboardType: const TextInputType.numberWithOptions(
                        decimal: true,
                      ),
                      decoration: const InputDecoration(
                        labelText: '最低消费 (选填，单位：元)',
                        hintText: '如：200',
                        border: OutlineInputBorder(),
                        prefixText: '¥ ',
                      ),
                      validator: (value) {
                        if (value != null && value.trim().isNotEmpty) {
                          if (double.tryParse(value.trim()) == null) {
                            return '请输入有效的金额';
                          }
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: AppSpacing.lg),

                    // Access Code Input
                    TextFormField(
                      controller: _accessCodeController,
                      obscureText: true,
                      decoration: InputDecoration(
                        labelText: widget.existingTable != null
                            ? '修改访问密码 (留空不修改)'
                            : '访问密码 (选填)',
                        hintText: '4-32位密码',
                        border: const OutlineInputBorder(),
                      ),
                      validator: (value) {
                        if (value != null && value.trim().isNotEmpty) {
                          if (value.trim().length < 4) {
                            return '密码至少4位';
                          }
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: AppSpacing.lg),

                    // Image upload
                    const Text(
                      '桌台图片 (选填)',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    _buildImagePicker(),
                    const SizedBox(height: AppSpacing.lg),

                    // Tags selection
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('桌台标签 (选填)'),
                      subtitle: Text(
                        _selectedTagIds.isEmpty
                            ? '未选择标签'
                            : '已选择 ${_selectedTagIds.length} 个标签',
                      ),
                      trailing: const Icon(Icons.chevron_right),
                      onTap: () {
                        showModalBottomSheet(
                          context: context,
                          backgroundColor: Colors.transparent,
                          builder: (context) => TableTagSelectorSheet(
                            initialSelectedTagIds: _selectedTagIds,
                            onSave: (tags) {
                              setState(() {
                                _selectedTagIds = tags;
                              });
                            },
                          ),
                        );
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
                                content: const Text(
                                  '确定要删除此桌台吗？如果有进行中的订单将无法删除。',
                                ),
                                actions: [
                                  TextButton(
                                    onPressed: () =>
                                        Navigator.pop(context, false),
                                    child: const Text('取消'),
                                  ),
                                  TextButton(
                                    onPressed: () =>
                                        Navigator.pop(context, true),
                                    style: TextButton.styleFrom(
                                      foregroundColor: AppColors.danger,
                                    ),
                                    child: const Text('确定删除'),
                                  ),
                                ],
                              ),
                            );

                            if (confirm == true && context.mounted) {
                              setState(() => _isSubmitting = true);
                              final success = await ref
                                  .read(tableProvider.notifier)
                                  .deleteTable(widget.existingTable!.id);
                              if (!context.mounted) return;
                              setState(() => _isSubmitting = false);

                              if (success) {
                                Navigator.pop(context);
                              } else {
                                final error =
                                    ref.read(tableProvider).error ?? '删除失败';
                                ScaffoldMessenger.of(
                                  context,
                                ).showSnackBar(SnackBar(content: Text(error)));
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
                    ],
                  ],
                ),
              ),
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
          color: isSelected
              ? AppColors.primary.withValues(alpha: 0.1)
              : AppColors.surfaceLow,
          border: Border.all(
            color: isSelected ? AppColors.primary : Colors.transparent,
            width: 2,
          ),
          borderRadius: BorderRadius.circular(AppRadius.lg),
        ),
        child: Column(
          children: [
            Icon(
              icon,
              color: isSelected
                  ? AppColors.primary
                  : AppColors.onSurfaceVariant,
            ),
            const SizedBox(height: 4),
            Text(
              type.label,
              style: TextStyle(
                color: isSelected
                    ? AppColors.primary
                    : AppColors.onSurfaceVariant,
                fontWeight: isSelected ? FontWeight.w700 : FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildImagePicker() {
    // 编辑模式下已有主图时显示
    final existingImageUrl = widget.existingTable?.imageUrl;
    final hasExisting = existingImageUrl != null && existingImageUrl.isNotEmpty;
    final hasUploaded = _uploadedImageUrl != null;

    if (hasUploaded) {
      return Stack(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.lg),
            child: Image.network(
              _uploadedImageUrl!,
              width: double.infinity,
              height: 120,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) => Container(
                height: 120,
                color: AppColors.surfaceLow,
                child: const Center(child: Icon(Icons.broken_image)),
              ),
            ),
          ),
          Positioned(
            top: 4,
            right: 4,
            child: GestureDetector(
              onTap: () => setState(() {
                _uploadedMediaAssetId = null;
                _uploadedImageUrl = null;
              }),
              child: Container(
                padding: const EdgeInsets.all(4),
                decoration: const BoxDecoration(
                  color: Colors.black54,
                  shape: BoxShape.circle,
                ),
                child: const Icon(Icons.close, color: Colors.white, size: 16),
              ),
            ),
          ),
        ],
      );
    }

    if (hasExisting) {
      return Stack(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.lg),
            child: Image.network(
              existingImageUrl,
              width: double.infinity,
              height: 120,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) => Container(
                height: 120,
                color: AppColors.surfaceLow,
                child: const Center(child: Icon(Icons.broken_image)),
              ),
            ),
          ),
          Positioned(
            bottom: 8,
            right: 8,
            child: MerchantImagePicker(
              businessType: 'table',
              mediaCategory: 'table',
              onSuccess: (mediaAssetId, url) {
                setState(() {
                  _uploadedMediaAssetId = mediaAssetId;
                  _uploadedImageUrl = url;
                });
              },
              onError: (error) {
                if (mounted) {
                  ScaffoldMessenger.of(
                    context,
                  ).showSnackBar(SnackBar(content: Text('上传失败: $error')));
                }
              },
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 10,
                  vertical: 6,
                ),
                decoration: BoxDecoration(
                  color: Colors.black54,
                  borderRadius: BorderRadius.circular(AppRadius.md),
                ),
                child: const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.edit, color: Colors.white, size: 14),
                    SizedBox(width: 4),
                    Text(
                      '更换图片',
                      style: TextStyle(color: Colors.white, fontSize: 12),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      );
    }

    return MerchantImagePicker(
      businessType: 'table',
      mediaCategory: 'table',
      onSuccess: (mediaAssetId, url) {
        setState(() {
          _uploadedMediaAssetId = mediaAssetId;
          _uploadedImageUrl = url;
        });
      },
      onError: (error) {
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('上传失败: $error')));
        }
      },
      child: Container(
        width: double.infinity,
        height: 100,
        decoration: BoxDecoration(
          color: AppColors.surfaceLow,
          borderRadius: BorderRadius.circular(AppRadius.lg),
          border: Border.all(
            color: AppColors.outlineVariant,
            style: BorderStyle.solid,
          ),
        ),
        child: const Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.add_photo_alternate_outlined,
              size: 32,
              color: AppColors.onSurfaceVariant,
            ),
            SizedBox(height: AppSpacing.xs),
            Text(
              '点击上传图片',
              style: TextStyle(fontSize: 12, color: AppColors.onSurfaceVariant),
            ),
          ],
        ),
      ),
    );
  }
}
