import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/providers/table_tag_provider.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

class TableTagSelectorSheet extends ConsumerStatefulWidget {
  final List<int> initialSelectedTagIds;
  final ValueChanged<List<int>> onSave;

  const TableTagSelectorSheet({
    super.key,
    required this.initialSelectedTagIds,
    required this.onSave,
  });

  @override
  ConsumerState<TableTagSelectorSheet> createState() =>
      _TableTagSelectorSheetState();
}

class _TableTagSelectorSheetState extends ConsumerState<TableTagSelectorSheet> {
  late Set<int> _selectedTagIds;

  @override
  void initState() {
    super.initState();
    _selectedTagIds = Set.from(widget.initialSelectedTagIds);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(tableTagProvider.notifier).fetchAvailableTags();
    });
  }

  void _toggleTag(int tagId) {
    setState(() {
      if (_selectedTagIds.contains(tagId)) {
        _selectedTagIds.remove(tagId);
      } else {
        _selectedTagIds.add(tagId);
      }
    });
  }

  void _handleSave() {
    widget.onSave(_selectedTagIds.toList());
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    final tagState = ref.watch(tableTagProvider);

    return Container(
      padding: const EdgeInsets.all(AppSpacing.xl),
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.xxl),
        ),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text(
                  '选择标签',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                ),
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.lg),
            if (tagState.isLoading)
              const SizedBox(
                height: 120,
                child: Center(child: CircularProgressIndicator()),
              )
            else if (tagState.error != null)
              SizedBox(
                height: 120,
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        tagState.error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: AppSpacing.md),
                      TextButton(
                        onPressed: () => ref
                            .read(tableTagProvider.notifier)
                            .fetchAvailableTags(),
                        child: const Text('重试'),
                      ),
                    ],
                  ),
                ),
              )
            else if (tagState.availableTags.isEmpty)
              const SizedBox(
                height: 120,
                child: Center(
                  child: Text(
                    '暂无可用标签',
                    style: TextStyle(color: AppColors.onSurfaceVariant),
                  ),
                ),
              )
            else
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: tagState.availableTags.map((tag) {
                  final isSelected = _selectedTagIds.contains(tag.id);
                  return ChoiceChip(
                    label: Text(tag.name),
                    selected: isSelected,
                    onSelected: (_) => _toggleTag(tag.id),
                    selectedColor: AppColors.positiveSoft,
                    labelStyle: TextStyle(
                      color: isSelected
                          ? AppColors.positive
                          : AppColors.onSurface,
                      fontWeight: isSelected
                          ? FontWeight.w600
                          : FontWeight.normal,
                    ),
                    side: BorderSide(
                      color: isSelected
                          ? AppColors.positive
                          : AppColors.outlineVariant,
                    ),
                    backgroundColor: Colors.transparent,
                  );
                }).toList(),
              ),
            const SizedBox(height: AppSpacing.xxl),
            MerchantPrimaryButton(
              label: '确定',
              expand: true,
              onPressed: _handleSave,
            ),
          ],
        ),
      ),
    );
  }
}
