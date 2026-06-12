import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/push/local_notification_service.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/core/service/navigation_service.dart';
import 'package:merchant_app/core/service/order_alert_checkpoint_store.dart';
import 'package:merchant_app/core/service/pending_order_alert_store.dart';
import 'package:merchant_app/features/display_config/display_config_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';
import 'package:merchant_app/features/auth/auth_state.dart';
import 'package:merchant_app/features/order/order_acceptance_coordinator.dart';
import 'package:merchant_app/features/order/order_alert_page.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';
import 'package:merchant_app/features/order/order_detail_page.dart';
import 'package:merchant_app/features/printer/local_print_event_repository.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  group('OrderAlertCoordinator.identifyNewAwaitingAcceptanceOrders', () {
    test(
      'only returns paid orders that were not awaiting acceptance before',
      () {
        final previousOrders = [
          _buildOrder(id: 'order-1', status: OrderStatus.paid),
          _buildOrder(id: 'order-2', status: OrderStatus.preparing),
        ];

        final latestOrders = [
          _buildOrder(id: 'order-1', status: OrderStatus.paid),
          _buildOrder(id: 'order-2', status: OrderStatus.paid),
          _buildOrder(id: 'order-3', status: OrderStatus.paid),
          _buildOrder(id: 'order-4', status: OrderStatus.pending),
          _buildOrder(id: 'order-5', status: OrderStatus.preparing),
        ];

        final result =
            OrderAlertCoordinator.identifyNewAwaitingAcceptanceOrders(
              previousOrders: previousOrders,
              latestOrders: latestOrders,
            );

        expect(result.map((order) => order.id).toList(), [
          'order-2',
          'order-3',
        ]);
      },
    );
  });

  group('OrderAlertCoordinator.handleIncomingOrder', () {
    test(
      'continues alert flow when local notification display fails',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              _ThrowingLocalNotificationService(),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await expectLater(
          coordinator.handleIncomingOrder(
            PushMessage(
              messageId: 'merchant:new_order:501',
              orderId: '501',
              orderNumber: 'ORD501',
              title: '新订单',
              content: '您有一笔新订单',
              amount: 18.5,
              shopName: '测试门店',
            ),
            showLocalNotification: true,
          ),
          completes,
        );
      },
    );

    test(
      'queues alert when navigator is unavailable without committing dedup',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final deduplicator = MessageDeduplicator.memoryOnly();
        await deduplicator.ensureInitialized();
        final pendingStore = MemoryPendingOrderAlertStore();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(deduplicator),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await coordinator.handleIncomingOrder(
          PushMessage(
            messageId: 'merchant:new_order:502',
            orderId: '502',
            orderNumber: 'ORD502',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
          showLocalNotification: false,
        );

        final pending = await pendingStore.loadPendingAlerts();
        expect(pending.map((alert) => alert.orderId), contains('502'));
        expect(
          await deduplicator.tryAcceptGroup([
            MessageDeduplicator.messageKey('merchant:new_order:502'),
            MessageDeduplicator.orderKey('502'),
          ]),
          isTrue,
        );
      },
    );

    test(
      'does not auto accept when backend display config disables it',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final pendingStore = MemoryPendingOrderAlertStore();
        final apiClient = _AutoAcceptApiClient();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              _CountingLocalNotificationService(),
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
            orderDisplayConfigRepositoryProvider.overrideWithValue(
              _FakeOrderDisplayConfigRepository(
                const OrderDisplayConfig(
                  enablePrint: true,
                  autoAcceptPaidOrders: false,
                ),
              ),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await coordinator.handleIncomingOrder(
          PushMessage(
            messageId: 'merchant:new_order:509',
            orderId: '509',
            orderNumber: 'ORD509',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
          showLocalNotification: false,
        );

        expect(apiClient.acceptPostCalls, 0);
        final pendingAlerts = await pendingStore.loadPendingAlerts();
        expect(pendingAlerts.map((alert) => alert.orderId), ['509']);
      },
    );

    test(
      'does not auto accept when backend disables printing even if auto accept is true',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final pendingStore = MemoryPendingOrderAlertStore();
        final apiClient = _AutoAcceptApiClient();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              _CountingLocalNotificationService(),
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
            orderDisplayConfigRepositoryProvider.overrideWithValue(
              _FakeOrderDisplayConfigRepository(
                const OrderDisplayConfig(
                  enablePrint: false,
                  autoAcceptPaidOrders: true,
                ),
              ),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await coordinator.handleIncomingOrder(
          PushMessage(
            messageId: 'merchant:new_order:512',
            orderId: '512',
            orderNumber: 'ORD512',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
          showLocalNotification: false,
        );

        expect(apiClient.acceptPostCalls, 0);
        final pendingAlerts = await pendingStore.loadPendingAlerts();
        expect(pendingAlerts.map((alert) => alert.orderId), ['512']);
      },
    );

    test(
      'auto accepts only when backend display config authorizes it',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final pendingStore = MemoryPendingOrderAlertStore();
        final checkpointStore = MemoryOrderAlertCheckpointStore();
        final apiClient = _AutoAcceptApiClient();
        final deduplicator = MessageDeduplicator.memoryOnly();
        await deduplicator.ensureInitialized();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            messageDeduplicatorProvider.overrideWithValue(deduplicator),
            localNotificationServiceProvider.overrideWithValue(
              _CountingLocalNotificationService(),
            ),
            orderAlertCheckpointStoreProvider.overrideWithValue(
              checkpointStore,
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
            orderDisplayConfigRepositoryProvider.overrideWithValue(
              _FakeOrderDisplayConfigRepository(
                const OrderDisplayConfig(
                  enablePrint: true,
                  autoAcceptPaidOrders: true,
                ),
              ),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await coordinator.handleIncomingOrder(
          PushMessage(
            messageId: 'merchant:new_order:510',
            orderId: '510',
            orderNumber: 'ORD510',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
          showLocalNotification: false,
        );

        expect(apiClient.acceptPostCalls, 1);
        expect(await pendingStore.loadPendingAlerts(), isEmpty);
        expect(await checkpointStore.hasAlerted('510'), isTrue);
        expect(
          await deduplicator.tryAcceptGroup([
            MessageDeduplicator.messageKey('merchant:new_order:510'),
            MessageDeduplicator.orderKey('510'),
          ]),
          isFalse,
        );
      },
    );

    test(
      'fails closed quickly when backend display config read hangs',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final pendingStore = MemoryPendingOrderAlertStore();
        final apiClient = _AutoAcceptApiClient();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              _CountingLocalNotificationService(),
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
            orderAlertDisplayConfigTimeoutProvider.overrideWithValue(
              const Duration(milliseconds: 1),
            ),
            orderDisplayConfigRepositoryProvider.overrideWithValue(
              _HangingOrderDisplayConfigRepository(),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);

        await coordinator.handleIncomingOrder(
          PushMessage(
            messageId: 'merchant:new_order:511',
            orderId: '511',
            orderNumber: 'ORD511',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
          showLocalNotification: false,
        );

        expect(apiClient.acceptPostCalls, 0);
        final pendingAlerts = await pendingStore.loadPendingAlerts();
        expect(pendingAlerts.map((alert) => alert.orderId), ['511']);
      },
    );

    test('fails closed when backend display config read throws', () async {
      TestWidgetsFlutterBinding.ensureInitialized();
      SharedPreferences.setMockInitialValues(<String, Object>{});
      final pendingStore = MemoryPendingOrderAlertStore();
      final apiClient = _AutoAcceptApiClient();
      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          messageDeduplicatorProvider.overrideWithValue(
            MessageDeduplicator.memoryOnly(),
          ),
          localNotificationServiceProvider.overrideWithValue(
            _CountingLocalNotificationService(),
          ),
          pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          notificationSettingsProvider.overrideWith(
            (ref) => _FakeNotificationSettingsNotifier(),
          ),
          orderDisplayConfigRepositoryProvider.overrideWithValue(
            _ThrowingOrderDisplayConfigRepository(),
          ),
        ],
      );
      addTearDown(container.dispose);

      final coordinator = container.read(orderAlertCoordinatorProvider);

      await coordinator.handleIncomingOrder(
        PushMessage(
          messageId: 'merchant:new_order:513',
          orderId: '513',
          orderNumber: 'ORD513',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        ),
        showLocalNotification: false,
      );

      expect(apiClient.acceptPostCalls, 0);
      final pendingAlerts = await pendingStore.loadPendingAlerts();
      expect(pendingAlerts.map((alert) => alert.orderId), ['513']);
    });
  });

  group('OrderAlertCoordinator.drainPendingAlerts', () {
    testWidgets(
      'presents pending alert once navigator is available and commits it',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final deduplicator = MessageDeduplicator.memoryOnly();
        await deduplicator.ensureInitialized();
        final checkpointStore = MemoryOrderAlertCheckpointStore();
        final pendingStore = MemoryPendingOrderAlertStore();
        final message = PushMessage(
          messageId: 'merchant:new_order:503',
          orderId: '503',
          orderNumber: 'ORD503',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );
        await pendingStore.save(message, source: 'incoming');

        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(deduplicator),
            orderAlertCheckpointStoreProvider.overrideWithValue(
              checkpointStore,
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          ],
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              navigatorKey: rootNavigatorKey,
              home: const Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );

        await container
            .read(orderAlertCoordinatorProvider)
            .drainPendingAlerts();
        await tester.pumpAndSettle();

        expect(find.byType(OrderAlertPage), findsOneWidget);
        expect(find.text('订单号 ORD503'), findsOneWidget);
        expect(await pendingStore.loadPendingAlerts(), isEmpty);
        expect(await checkpointStore.hasAlerted('503'), isTrue);
        expect(
          await deduplicator.tryAcceptGroup([
            MessageDeduplicator.messageKey('merchant:new_order:503'),
            MessageDeduplicator.orderKey('503'),
          ]),
          isFalse,
        );

        await tester.pumpWidget(const SizedBox.shrink());
      },
    );

    testWidgets(
      'does not drop pending alert just because dedup was committed earlier',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final deduplicator = MessageDeduplicator.memoryOnly();
        await deduplicator.ensureInitialized();
        final checkpointStore = MemoryOrderAlertCheckpointStore();
        final pendingStore = MemoryPendingOrderAlertStore();
        final message = PushMessage(
          messageId: 'merchant:new_order:504',
          orderId: '504',
          orderNumber: 'ORD504',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );
        await pendingStore.save(message, source: 'incoming');
        await deduplicator.markAccepted([
          MessageDeduplicator.messageKey(message.messageId),
          MessageDeduplicator.orderKey(message.orderId),
        ]);

        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(deduplicator),
            orderAlertCheckpointStoreProvider.overrideWithValue(
              checkpointStore,
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          ],
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              navigatorKey: rootNavigatorKey,
              home: const Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );

        await container
            .read(orderAlertCoordinatorProvider)
            .drainPendingAlerts();
        await tester.pumpAndSettle();

        expect(find.byType(OrderAlertPage), findsOneWidget);
        expect(find.text('订单号 ORD504'), findsOneWidget);
        expect(await pendingStore.loadPendingAlerts(), isEmpty);
        expect(await checkpointStore.hasAlerted('504'), isTrue);

        await tester.pumpWidget(const SizedBox.shrink());
      },
    );

    testWidgets(
      'removes pending alert without presenting when backend order is no longer paid',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final pendingStore = MemoryPendingOrderAlertStore();
        final checkpointStore = MemoryOrderAlertCheckpointStore();
        final message = PushMessage(
          messageId: 'merchant:new_order:506',
          orderId: '506',
          orderNumber: 'ORD506',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );
        await pendingStore.save(message, source: 'incoming');

        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(
              _OrderDetailApiClient(_orderJson(id: '506', status: 'preparing')),
            ),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            orderAlertCheckpointStoreProvider.overrideWithValue(
              checkpointStore,
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          ],
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              navigatorKey: rootNavigatorKey,
              home: const Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );

        await container
            .read(orderAlertCoordinatorProvider)
            .drainPendingAlerts();
        await tester.pumpAndSettle();

        expect(find.byType(OrderAlertPage), findsNothing);
        expect(await pendingStore.loadPendingAlerts(), isEmpty);
        expect(await checkpointStore.hasAlerted('506'), isFalse);

        await tester.pumpWidget(const SizedBox.shrink());
      },
    );
  });

  group('OrderAlertCoordinator.handleNotificationTap', () {
    testWidgets(
      'queues notification tap until navigator is ready and then shows alert for paid order',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(
              _OrderDetailApiClient(_orderJson(id: '509', status: 'paid')),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);
        await coordinator.handleNotificationTap(
          PushMessage(
            messageId: 'merchant:new_order:509',
            orderId: '509',
            orderNumber: 'ORD509',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
        );

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              navigatorKey: rootNavigatorKey,
              home: const Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );

        await coordinator.drainPendingNotificationTaps();
        await tester.pumpAndSettle();

        expect(find.byType(OrderAlertPage), findsOneWidget);
        expect(find.text('订单号 ORD509'), findsOneWidget);

        await tester.pumpWidget(const SizedBox.shrink());
      },
    );

    testWidgets(
      'queues notification tap until navigator is ready and then opens detail for non-paid order',
      (tester) async {
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(
              _OrderDetailApiClient(_orderJson(id: '510', status: 'preparing')),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);
        await coordinator.handleNotificationTap(
          PushMessage(
            messageId: 'merchant:new_order:510',
            orderId: '510',
            orderNumber: 'ORD510',
            title: '新订单',
            content: '您有一笔新订单',
            amount: 18.5,
            shopName: '测试门店',
          ),
        );

        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              navigatorKey: rootNavigatorKey,
              home: const Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );

        await coordinator.drainPendingNotificationTaps();
        await tester.pumpAndSettle();

        expect(find.byType(OrderDetailPage), findsOneWidget);
        expect(find.text('订单 ORD510'), findsOneWidget);

        await tester.pumpWidget(const SizedBox.shrink());
      },
    );
  });

  group('OrderAlertCoordinator duplicate delivery', () {
    test(
      'suppresses duplicate side effects while the same order is already being handled',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final localNotificationService = _BlockingLocalNotificationService();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              localNotificationService,
            ),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);
        final message = PushMessage(
          messageId: 'merchant:new_order:507',
          orderId: '507',
          orderNumber: 'ORD507',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );

        final first = coordinator.handleIncomingOrder(
          message,
          showLocalNotification: true,
        );
        await Future<void>.delayed(Duration.zero);

        final second = coordinator.handleIncomingOrder(
          message,
          showLocalNotification: true,
        );
        await Future<void>.delayed(Duration.zero);

        expect(localNotificationService.showCount, 1);

        localNotificationService.complete();
        await Future.wait([first, second]);
      },
    );

    test(
      'suppresses duplicate side effects while an order is pending presentation retry',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final localNotificationService = _CountingLocalNotificationService();
        final pendingStore = MemoryPendingOrderAlertStore();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              localNotificationService,
            ),
            pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);
        final message = PushMessage(
          messageId: 'merchant:new_order:508',
          orderId: '508',
          orderNumber: 'ORD508',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );

        final polledDuplicate = PushMessage(
          messageId: 'polled-508',
          orderId: '508',
          orderNumber: 'ORD508',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );

        await coordinator.handleIncomingOrder(
          polledDuplicate,
          showLocalNotification: true,
        );
        await coordinator.handleIncomingOrder(
          message,
          showLocalNotification: true,
        );

        expect(localNotificationService.showCount, 1);
        final pendingAlerts = await pendingStore.loadPendingAlerts();
        expect(pendingAlerts.map((alert) => alert.orderId), ['508']);
      },
    );

    test(
      'coalesces duplicate auto-accept delivery without double printing receipts',
      () async {
        TestWidgetsFlutterBinding.ensureInitialized();
        SharedPreferences.setMockInitialValues(<String, Object>{});
        final localNotificationService = _BlockingLocalNotificationService();
        final apiClient = _AutoAcceptApiClient();
        final receiptPrinter = _CountingAcceptedOrderReceiptPrinter();
        final localPrintEvents = _CountingLocalPrintEventRepository();
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            messageDeduplicatorProvider.overrideWithValue(
              MessageDeduplicator.memoryOnly(),
            ),
            localNotificationServiceProvider.overrideWithValue(
              localNotificationService,
            ),
            acceptedOrderReceiptPrinterProvider.overrideWithValue(
              receiptPrinter,
            ),
            localPrintEventRepositoryProvider.overrideWithValue(
              localPrintEvents,
            ),
            notificationSettingsProvider.overrideWith(
              (ref) => _FakeNotificationSettingsNotifier(
                autoPrintAfterAcceptEnabled: true,
              ),
            ),
            orderDisplayConfigRepositoryProvider.overrideWithValue(
              _FakeOrderDisplayConfigRepository(
                const OrderDisplayConfig(
                  enablePrint: true,
                  autoAcceptPaidOrders: true,
                ),
              ),
            ),
          ],
        );
        addTearDown(container.dispose);

        final coordinator = container.read(orderAlertCoordinatorProvider);
        final websocketMessage = PushMessage(
          messageId: 'ws:new_order:514',
          orderId: '514',
          orderNumber: 'ORD514',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );
        final polledDuplicate = PushMessage(
          messageId: 'poll:new_order:514',
          orderId: '514',
          orderNumber: 'ORD514',
          title: '新订单',
          content: '您有一笔新订单',
          amount: 18.5,
          shopName: '测试门店',
        );

        final first = coordinator.handleIncomingOrder(
          websocketMessage,
          showLocalNotification: true,
        );
        await Future<void>.delayed(Duration.zero);

        final second = coordinator.handleIncomingOrder(
          polledDuplicate,
          showLocalNotification: true,
        );
        await Future<void>.delayed(Duration.zero);

        expect(localNotificationService.showCount, 1);

        localNotificationService.complete();
        await Future.wait([first, second]);

        expect(apiClient.acceptPostCalls, 1);
        expect(receiptPrinter.printedOrderIds, ['514']);
        expect(localPrintEvents.statusesForOrder('514'), [
          'started',
          'success',
        ]);
      },
    );
  });

  group('PendingOrderAlertDrainManager', () {
    testWidgets('drains pending alerts after the first app frame', (
      tester,
    ) async {
      SharedPreferences.setMockInitialValues(<String, Object>{});
      final deduplicator = MessageDeduplicator.memoryOnly();
      await deduplicator.ensureInitialized();
      final checkpointStore = MemoryOrderAlertCheckpointStore();
      final pendingStore = MemoryPendingOrderAlertStore();
      final message = PushMessage(
        messageId: 'merchant:new_order:505',
        orderId: '505',
        orderNumber: 'ORD505',
        title: '新订单',
        content: '您有一笔新订单',
        amount: 18.5,
        shopName: '测试门店',
      );
      await pendingStore.save(message, source: 'incoming');

      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(_FakeApiClient()),
          messageDeduplicatorProvider.overrideWithValue(deduplicator),
          orderAlertCheckpointStoreProvider.overrideWithValue(checkpointStore),
          pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            navigatorKey: rootNavigatorKey,
            home: Consumer(
              builder: (context, ref, child) {
                ref.watch(pendingOrderAlertDrainManagerProvider);
                return const Scaffold(body: SizedBox.shrink());
              },
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(OrderAlertPage), findsOneWidget);
      expect(find.text('订单号 ORD505'), findsOneWidget);
      expect(await pendingStore.loadPendingAlerts(), isEmpty);
      expect(await checkpointStore.hasAlerted('505'), isTrue);

      await tester.pumpWidget(const SizedBox.shrink());
    });
  });

  group('PushMessage.fromOrder', () {
    test('keeps real id for actions and order number for display', () {
      final order = _buildOrder(
        id: 'internal-id-1',
        orderNum: 'LKLF-20260412-001',
        status: OrderStatus.paid,
      );

      final message = PushMessage.fromOrder(order, shopName: '测试门店');

      expect(message.orderId, 'internal-id-1');
      expect(message.displayOrderNumber, 'LKLF-20260412-001');
      expect(message.shopName, '测试门店');
    });

    test('uses pickup code as merchant-facing display number', () {
      final order = _buildOrder(
        id: 'internal-id-1',
        orderNum: 'ORD20260412000001',
        pickupCode: '0386',
        status: OrderStatus.paid,
      );

      final message = PushMessage.fromOrder(order, shopName: '测试门店');

      expect(message.orderId, 'internal-id-1');
      expect(message.orderNumber, 'ORD20260412000001');
      expect(message.displayOrderNumber, '0386');
    });
  });

  group('OrderModel.fromJson', () {
    test('normalizes backend order fields with item specs and notes', () {
      final order = OrderModel.fromJson({
        'id': 501,
        'order_no': 'ORD501',
        'total_amount': 8800,
        'status': 'paid',
        'created_at': '2026-04-12T08:00:00Z',
        'delivery_contact_name': '张三',
        'delivery_contact_phone': '13800138000',
        'notes': '少放葱',
        'fee_breakdown': {
          'customer_payable_amount': 8800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 31,
          'merchant_receivable_amount': 8294,
          'rider_gross_amount': 800,
          'rider_payment_fee_amount': 5,
          'rider_net_earnings_amount': 795,
        },
        'items': [
          {
            'id': 801,
            'name': '测试菜品',
            'quantity': 2,
            'unit_price': 2800,
            'subtotal': 5600,
            'specs_text': '大份 / 少辣',
          },
        ],
      });

      expect(order.id, '501');
      expect(order.orderNum, 'ORD501');
      expect(order.amount, 88.0);
      expect(order.status, OrderStatus.paid);
      expect(order.isAwaitingAcceptance, isTrue);
      expect(order.userName, '张三');
      expect(order.userPhone, '13800138000');
      expect(order.note, '少放葱');
      expect(order.hasReliableItems, isTrue);
      expect(order.feeBreakdown, isNotNull);
      expect(order.feeBreakdown!.customerPayableAmountCents, 8800);
      expect(order.feeBreakdown!.platformServiceFeeAmountCents, 475);
      expect(order.feeBreakdown!.paymentChannelFeeAmountCents, 31);
      expect(order.feeBreakdown!.merchantReceivableAmountCents, 8294);
      expect(order.feeBreakdown!.riderGrossAmountCents, 800);
      expect(order.feeBreakdown!.riderPaymentFeeCents, 5);
      expect(order.feeBreakdown!.riderNetEarningsCents, 795);
      expect(order.items.single.price, 28.0);
      expect(order.items.single.subtotal, 56.0);
      expect(order.items.single.specsText, '大份 / 少辣');
    });

    test('prefers backend pickup code for merchant display number', () {
      final order = OrderModel.fromJson({
        'id': 501,
        'order_no': 'ORD20260412000001',
        'pickup_code': '0386',
        'pickup_code_masked': '03**',
        'total_amount': 8800,
        'status': 'paid',
        'created_at': '2026-04-12T08:00:00Z',
      });

      expect(order.orderNum, 'ORD20260412000001');
      expect(order.pickupCode, '0386');
      expect(order.pickupCodeMasked, '03**');
      expect(order.displayOrderNumber, '0386');
    });

    test('parses merchant fee breakdown from backend cents', () {
      final order = OrderModel.fromJson({
        'id': 501,
        'order_no': 'ORD501',
        'total_amount': 10300,
        'status': 'paid',
        'created_at': '2026-04-12T08:00:00Z',
        'fee_breakdown': {
          'food_amount': 10000,
          'merchant_discount_amount': 300,
          'voucher_discount_amount': 200,
          'food_payable_amount': 9500,
          'delivery_fee_amount': 800,
          'delivery_fee_discount_amount': 0,
          'delivery_payable_amount': 800,
          'customer_payable_amount': 10300,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 57,
          'merchant_receivable_amount': 8968,
        },
      });

      expect(order.amount, 103.0);
      expect(order.feeBreakdown, isNotNull);
      expect(order.feeBreakdown!.foodPayableAmount, 95.0);
      expect(order.feeBreakdown!.deliveryPayableAmount, 8.0);
      expect(order.feeBreakdown!.platformServiceFeeAmount, 4.75);
      expect(order.feeBreakdown!.paymentChannelFeeAmount, 0.57);
      expect(order.feeBreakdown!.merchantReceivableAmount, 89.68);
    });

    test('keeps legacy aliases compatible', () {
      final order = OrderModel.fromJson({
        'order_id': 'legacy-1',
        'order_num': 'LEGACY001',
        'amount': 18.5,
        'status': 'pending',
        'created_at': '2026-04-12T08:00:00Z',
        'note': '不要香菜',
        'items_load_failed': true,
        'items': [
          {'name': '旧菜品', 'quantity': 1, 'price': 18.5},
        ],
      });

      expect(order.id, 'legacy-1');
      expect(order.orderNum, 'LEGACY001');
      expect(order.amount, 18.5);
      expect(order.status, OrderStatus.pending);
      expect(order.isAwaitingAcceptance, isFalse);
      expect(order.note, '不要香菜');
      expect(order.hasReliableItems, isFalse);
      expect(order.items.single.price, 18.5);
    });

    test(
      'does not default missing or unknown backend status to accept action',
      () {
        final missingStatusOrder = OrderModel.fromJson({
          'id': 'missing-status',
          'order_no': 'ORD-MISSING',
          'total_amount': 1800,
          'created_at': '2026-04-12T08:00:00Z',
        });
        final unknownStatusOrder = OrderModel.fromJson({
          'id': 'unknown-status',
          'order_no': 'ORD-UNKNOWN',
          'total_amount': 1800,
          'status': 'reviewing',
          'created_at': '2026-04-12T08:00:00Z',
        });

        expect(missingStatusOrder.status, OrderStatus.unknown);
        expect(missingStatusOrder.isAwaitingAcceptance, isFalse);
        expect(unknownStatusOrder.status, OrderStatus.unknown);
        expect(unknownStatusOrder.isAwaitingAcceptance, isFalse);
      },
    );

    test(
      'allows mark-ready while rider has accepted but kitchen is preparing',
      () {
        final order = OrderModel.fromJson({
          'id': 501,
          'order_no': 'ORD501',
          'total_amount': 1800,
          'status': 'courier_accepted',
          'fulfillment_status': 'preparing',
          'created_at': '2026-04-12T08:00:00Z',
        });

        expect(order.canMarkReady, isTrue);
      },
    );
  });

  group('PushMessage.fromJson', () {
    test('carries backend snapshot items and specs', () {
      final message = PushMessage.fromJson({
        'message_id': 'merchant:new_order:501',
        'order_id': 501,
        'order_no': 'ORD501',
        'title': '新订单',
        'content': '您有一笔新订单 ORD501，请及时处理',
        'amount': 8800,
        'shop_name': '测试门店',
        'notes': '少放葱',
        'fee_breakdown': {
          'customer_payable_amount': 8800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 31,
          'merchant_receivable_amount': 8294,
          'rider_gross_amount': 800,
          'rider_payment_fee_amount': 5,
          'rider_net_earnings_amount': 795,
        },
        'items': [
          {
            'name': '测试菜品',
            'quantity': 2,
            'unit_price': 2800,
            'subtotal': 5600,
            'specs_text': '大份 / 少辣',
          },
        ],
      });

      expect(message.messageId, 'merchant:new_order:501');
      expect(message.orderId, '501');
      expect(message.displayOrderNumber, 'ORD501');
      expect(message.amount, 88.0);
      expect(message.note, '少放葱');
      expect(message.feeBreakdown, isNotNull);
      expect(message.feeBreakdown!.merchantReceivableAmountCents, 8294);
      expect(message.feeBreakdown!.riderGrossAmountCents, 800);
      expect(message.feeBreakdown!.riderPaymentFeeCents, 5);
      expect(message.feeBreakdown!.riderNetEarningsCents, 795);
      expect(message.items.single.specsText, '大份 / 少辣');
      expect(message.itemsLoadFailed, isFalse);
    });

    test('uses pickup code from notification payload as display number', () {
      final message = PushMessage.fromJson({
        'message_id': 'merchant:new_order:501',
        'order_id': 501,
        'order_no': 'ORD20260412000001',
        'pickup_code': '0386',
        'pickup_code_masked': '03**',
        'amount': 8800,
        'shop_name': '测试门店',
      });

      expect(message.orderId, '501');
      expect(message.orderNumber, 'ORD20260412000001');
      expect(message.pickupCode, '0386');
      expect(message.pickupCodeMasked, '03**');
      expect(message.displayOrderNumber, '0386');
    });

    test('carries fee breakdown through push hydration', () {
      final message = PushMessage.fromJson({
        'message_id': 'merchant:new_order:501',
        'order_id': 501,
        'order_no': 'ORD501',
        'amount': 10300,
        'shop_name': '测试门店',
        'fee_breakdown': {
          'food_payable_amount': 9500,
          'delivery_payable_amount': 800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 57,
          'merchant_receivable_amount': 8968,
        },
      });

      expect(message.feeBreakdown, isNotNull);
      expect(message.feeBreakdown!.foodPayableAmount, 95.0);

      final hydrated = message.withOrderSnapshot(
        OrderModel.fromJson({
          'id': 501,
          'order_no': 'ORD501',
          'total_amount': 10300,
          'fee_breakdown': {
            'food_payable_amount': 9600,
            'delivery_payable_amount': 700,
            'platform_service_fee_amount': 480,
            'payment_channel_fee_amount': 58,
            'merchant_receivable_amount': 9062,
          },
        }),
      );

      expect(hydrated.feeBreakdown!.foodPayableAmount, 96.0);
      expect(hydrated.feeBreakdown!.deliveryPayableAmount, 7.0);
    });

    test(
      'keeps message identity when replacing snapshot from detail order',
      () {
        final message = PushMessage.fromJson({
          'message_id': 'merchant:new_order:501',
          'order_id': 501,
          'amount': 8800,
          'shop_name': '测试门店',
        });
        final order = OrderModel.fromJson({
          'id': 501,
          'order_no': 'ORD501',
          'total_amount': 8800,
          'notes': '少放葱',
          'fee_breakdown': {
            'customer_payable_amount': 8800,
            'platform_service_fee_amount': 475,
            'payment_channel_fee_amount': 31,
            'merchant_receivable_amount': 8294,
            'rider_gross_amount': 800,
            'rider_payment_fee_amount': 5,
            'rider_net_earnings_amount': 795,
          },
          'items': [
            {
              'name': '测试菜品',
              'quantity': 2,
              'unit_price': 2800,
              'subtotal': 5600,
              'specs_text': '大份 / 少辣',
            },
          ],
        });

        final hydrated = message.withOrderSnapshot(order);

        expect(hydrated.messageId, 'merchant:new_order:501');
        expect(hydrated.displayOrderNumber, 'ORD501');
        expect(hydrated.amount, 88.0);
        expect(hydrated.note, '少放葱');
        expect(hydrated.feeBreakdown, isNotNull);
        expect(hydrated.feeBreakdown!.merchantReceivableAmountCents, 8294);
        expect(hydrated.feeBreakdown!.riderGrossAmountCents, 800);
        expect(hydrated.feeBreakdown!.riderPaymentFeeCents, 5);
        expect(hydrated.feeBreakdown!.riderNetEarningsCents, 795);
        expect(hydrated.items.single.specsText, '大份 / 少辣');
      },
    );
  });

  group('OrderAlertPage', () {
    testWidgets('shows accept reject and defer actions together', (
      tester,
    ) async {
      final message = PushMessage(
        messageId: 'merchant:new_order:501',
        orderId: '501',
        orderNumber: 'ORD501',
        pickupCode: '0386',
        title: '新订单',
        content: '您有一笔新订单',
        amount: 103.0,
        feeBreakdown: const OrderFeeBreakdown(
          foodAmountCents: 10000,
          merchantDiscountAmountCents: 300,
          voucherDiscountAmountCents: 200,
          foodPayableAmountCents: 9500,
          deliveryFeeAmountCents: 800,
          deliveryFeeDiscountAmountCents: 0,
          deliveryPayableAmountCents: 800,
          customerPayableAmountCents: 10300,
          platformServiceFeeAmountCents: 475,
          paymentChannelFeeAmountCents: 57,
          merchantReceivableAmountCents: 8968,
        ),
        shopName: '测试门店',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [apiClientProvider.overrideWithValue(_FakeApiClient())],
          child: MaterialApp(home: OrderAlertPage(message: message)),
        ),
      );

      expect(find.text('立即接单'), findsOneWidget);
      expect(find.text('拒单'), findsOneWidget);
      expect(find.text('稍后处理'), findsOneWidget);
      expect(find.text('¥ 95.00'), findsOneWidget);
      expect(find.text('代取费另列 ¥8.00'), findsOneWidget);
      expect(find.text('订单号 0386'), findsOneWidget);
      expect(find.text('订单号 ORD501'), findsNothing);
    });
  });

  group('OrderDetailPage', () {
    testWidgets('shows mark-ready action for preparing orders', (tester) async {
      final authService = _FakeAuthService();
      final sessionController = AuthSessionController();
      final order = _buildOrder(
        id: '501',
        orderNum: 'ORD20260412000001',
        pickupCode: '0386',
        status: OrderStatus.preparing,
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(_FakeApiClient()),
            authServiceProvider.overrideWithValue(authService),
            authSessionControllerProvider.overrideWithValue(sessionController),
            authProvider.overrideWith(
              (ref) =>
                  AuthNotifier(authService, sessionController)
                    ..state = AuthState(
                      accessToken: 'access-token',
                      refreshToken: 'refresh-token',
                      merchantName: '测试门店',
                      isAuthenticated: true,
                    ),
            ),
          ],
          child: MaterialApp(home: OrderDetailPage(order: order)),
        ),
      );

      await tester.pump();

      expect(find.text('订单 0386'), findsOneWidget);
      expect(find.text('制作完成'), findsOneWidget);
      expect(find.text('立即接单'), findsNothing);
    });
  });
}

class _FakeApiClient implements ApiClient {
  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _ThrowingLocalNotificationService extends LocalNotificationService {
  @override
  Future<void> showNewOrderNotification(PushMessage message) async {
    throw StateError('notification display failed');
  }
}

class _BlockingLocalNotificationService extends LocalNotificationService {
  final Completer<void> _completer = Completer<void>();
  int showCount = 0;

  @override
  Future<void> showNewOrderNotification(PushMessage message) async {
    showCount += 1;
    return _completer.future;
  }

  void complete() {
    if (!_completer.isCompleted) {
      _completer.complete();
    }
  }
}

class _CountingLocalNotificationService extends LocalNotificationService {
  int showCount = 0;

  @override
  Future<void> showNewOrderNotification(PushMessage message) async {
    showCount += 1;
  }
}

class _OrderDetailApiClient implements ApiClient {
  _OrderDetailApiClient(this.orderJson);

  final Map<String, dynamic> orderJson;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    if (path == '/merchant/orders/${orderJson['id']}') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{'code': 0, 'message': 'ok', 'data': orderJson},
      );
    }
    throw UnimplementedError('Unexpected GET $path');
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _FakeNotificationSettingsNotifier
    extends StateNotifier<NotificationSettingsState>
    implements NotificationSettingsNotifier {
  _FakeNotificationSettingsNotifier({bool autoPrintAfterAcceptEnabled = false})
    : super(
        const NotificationSettingsState(
          soundEnabled: false,
          voiceEnabled: false,
        ).copyWith(autoPrintAfterAcceptEnabled: autoPrintAfterAcceptEnabled),
      );

  @override
  Future<void> setAutoPrintAfterAcceptEnabled(bool enabled) async {}

  @override
  Future<void> setSoundEnabled(bool enabled) async {}

  @override
  Future<void> setVoiceEnabled(bool enabled) async {}
}

class _CountingAcceptedOrderReceiptPrinter
    implements AcceptedOrderReceiptPrinter {
  final List<String> printedOrderIds = <String>[];

  @override
  bool get hasConnectedPrinter => true;

  @override
  String get printerName => '测试蓝牙打印机';

  @override
  Future<bool> printAcceptedOrder(
    OrderModel order, {
    required String shopName,
  }) async {
    printedOrderIds.add(order.id);
    return true;
  }
}

class _CountingLocalPrintEventRepository implements LocalPrintEventRepository {
  final List<_LocalPrintEventRecord> records = <_LocalPrintEventRecord>[];

  @override
  Future<void> recordAcceptedReceiptEvent(
    OrderModel order, {
    required String status,
    String? printerName,
    String? errorMessage,
  }) async {
    records.add(_LocalPrintEventRecord(order.id, status));
  }

  List<String> statusesForOrder(String orderId) {
    return records
        .where((record) => record.orderId == orderId)
        .map((record) => record.status)
        .toList();
  }
}

class _LocalPrintEventRecord {
  const _LocalPrintEventRecord(this.orderId, this.status);

  final String orderId;
  final String status;
}

class _FakeOrderDisplayConfigRepository
    implements OrderDisplayConfigRepository {
  _FakeOrderDisplayConfigRepository(this.config);

  final OrderDisplayConfig config;

  @override
  Future<OrderDisplayConfig> fetchDisplayConfig() async => config;
}

class _HangingOrderDisplayConfigRepository
    implements OrderDisplayConfigRepository {
  @override
  Future<OrderDisplayConfig> fetchDisplayConfig() =>
      Completer<OrderDisplayConfig>().future;
}

class _ThrowingOrderDisplayConfigRepository
    implements OrderDisplayConfigRepository {
  @override
  Future<OrderDisplayConfig> fetchDisplayConfig() {
    throw StateError('display config unavailable');
  }
}

class _AutoAcceptApiClient implements ApiClient {
  int acceptPostCalls = 0;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    if (path.startsWith('/merchant/orders/')) {
      final orderId = path.split('/').last;
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': _orderJson(id: orderId, status: 'paid'),
        },
      );
    }
    if (path == '/merchant/orders') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{
            'orders': <Map<String, dynamic>>[],
            'total': 0,
            'page_id': 1,
            'page_size': 20,
          },
        },
      );
    }
    throw UnimplementedError('Unexpected GET $path');
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    if (path.startsWith('/merchant/orders/') && path.endsWith('/accept')) {
      acceptPostCalls += 1;
      final orderId = path.split('/')[3];
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': _orderJson(id: orderId, status: 'preparing'),
        },
      );
    }
    throw UnimplementedError('Unexpected POST $path');
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _FakeAuthService implements AuthService {
  @override
  Future<void> clearTokens() async {}

  @override
  Future<Map<String, String?>> getTokens() async => <String, String?>{
    'accessToken': 'access-token',
    'refreshToken': 'refresh-token',
    'merchantName': '测试门店',
  };

  @override
  Future<Map<String, dynamic>> refreshToken(String refreshToken) async {
    return <String, dynamic>{
      'access_token': 'access-token',
      'refresh_token': 'refresh-token',
    };
  }

  @override
  Future<void> saveTokens(
    String access,
    String refresh, {
    String? merchantName,
  }) async {}

  @override
  Future<Map<String, String?>?> tryAutoLogin() async {
    return <String, String?>{
      'accessToken': 'access-token',
      'refreshToken': 'refresh-token',
      'merchantName': '测试门店',
    };
  }

  @override
  Future<Map<String, dynamic>> verifyBindingCode(String code) {
    throw UnimplementedError();
  }
}

OrderModel _buildOrder({
  required String id,
  required OrderStatus status,
  String orderNum = '',
  String? pickupCode,
  String? pickupCodeMasked,
}) {
  return OrderModel(
    id: id,
    orderNum: orderNum,
    pickupCode: pickupCode,
    pickupCodeMasked: pickupCodeMasked,
    amount: 18.5,
    status: status,
    createdAt: DateTime.parse('2026-04-12T08:00:00Z'),
    items: const <OrderItem>[],
  );
}

Map<String, dynamic> _orderJson({
  required String id,
  required String status,
  String orderType = 'takeout',
}) {
  return <String, dynamic>{
    'id': id,
    'order_no': 'ORD$id',
    'order_type': orderType,
    'total_amount': 1850,
    'status': status,
    'created_at': '2026-04-12T08:00:00Z',
    'items': <Map<String, dynamic>>[],
  };
}
