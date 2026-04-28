import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/ui/widgets/table_card.dart';
import 'package:merchant_app/features/table/ui/widgets/table_action_sheet.dart';
import 'package:merchant_app/features/table/ui/widgets/table_config_sheet.dart';

class TableGridScreen extends ConsumerStatefulWidget {
  const TableGridScreen({super.key});

  @override
  ConsumerState<TableGridScreen> createState() => _TableGridScreenState();
}

class _TableGridScreenState extends ConsumerState<TableGridScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(tableProvider.notifier).fetchTables();
    });
  }

  void _showTableActionSheet(BuildContext context, table) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => TableActionSheet(
        table: table,
        onUpdateStatus: (status) async {
          return await ref.read(tableProvider.notifier).updateTableStatus(table.id, status);
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final tableState = ref.watch(tableProvider);

    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('桌台管理'),
          bottom: const TabBar(
            tabs: [
              Tab(text: '大厅散座'),
              Tab(text: '包间'),
            ],
          ),
          actions: [
            IconButton(
              icon: const Icon(Icons.refresh),
              onPressed: () {
                ref.read(tableProvider.notifier).fetchTables();
              },
            ),
          ],
        ),
        body: tableState.isLoading && tableState.tables.isEmpty
            ? const Center(child: CircularProgressIndicator())
            : TabBarView(
                children: [
                  _buildGrid(
                    tableState.tables.where((t) => t.tableType == TableType.table).toList(),
                  ),
                  _buildGrid(
                    tableState.tables.where((t) => t.tableType == TableType.room).toList(),
                  ),
                ],
              ),
        floatingActionButton: FloatingActionButton(
          onPressed: () {
            showModalBottomSheet(
              context: context,
              isScrollControlled: true,
              backgroundColor: Colors.transparent,
              builder: (context) => const TableConfigSheet(),
            );
          },
          child: const Icon(Icons.add),
        ),
      ),
    );
  }

  Widget _buildGrid(List tables) {
    if (tables.isEmpty) {
      return Center(
        child: Text(
          '暂无桌台',
          style: TextStyle(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () async {
        await ref.read(tableProvider.notifier).fetchTables();
      },
      child: GridView.builder(
        padding: const EdgeInsets.all(AppSpacing.md),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 3,
          childAspectRatio: 1.0,
          crossAxisSpacing: AppSpacing.sm,
          mainAxisSpacing: AppSpacing.sm,
        ),
        itemCount: tables.length,
        itemBuilder: (context, index) {
          final table = tables[index];
          return TableCard(
            table: table,
            onTap: () => _showTableActionSheet(context, table),
          );
        },
      ),
    );
  }
}
