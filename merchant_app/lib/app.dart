import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/core/service/navigation_service.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/bind_code_page.dart';
import 'package:merchant_app/features/auth/landing_page.dart';
import 'package:merchant_app/features/order/order_list_page.dart';
import 'package:merchant_app/features/settings/settings_page.dart';
import 'package:merchant_app/features/settings/permission_guide_page.dart';
import 'package:merchant_app/features/settings/about_page.dart';
import 'package:merchant_app/features/settings/agreement_page.dart';
import 'package:merchant_app/features/printer/bluetooth_printer_page.dart';
import 'package:merchant_app/features/order/order_detail_page.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/table/ui/table_grid_screen.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/models/order.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final routerNotifier = ref.watch(routerNotifierProvider);

  return GoRouter(
    navigatorKey: rootNavigatorKey,
    initialLocation: '/',
    refreshListenable: routerNotifier,
    redirect: routerNotifier.redirect,
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => const OrderListPage(),
        routes: [
          GoRoute(
            path: 'order-detail',
            builder: (context, state) {
              final order = state.extra as OrderModel;
              return OrderDetailPage(order: order);
            },
          ),
        ],
      ),
      GoRoute(
        path: '/welcome',
        builder: (context, state) => const LandingPage(),
      ),
      GoRoute(
        path: '/login',
        builder: (context, state) => const BindCodePage(),
      ),
      GoRoute(
        path: '/tables',
        builder: (context, state) => const TableGridScreen(),
      ),

      GoRoute(
        path: '/settings',
        builder: (context, state) => const SettingsPage(),
        routes: [
          GoRoute(
            path: 'permission-guide',
            builder: (context, state) => const PermissionGuidePage(),
          ),
          GoRoute(
            path: 'about',
            builder: (context, state) => const AboutPage(),
          ),
          GoRoute(
            path: 'agreement/:type',
            builder: (context, state) => AgreementPage(
              agreementType: state.pathParameters['type'] ?? '',
            ),
          ),
        ],
      ),
      GoRoute(
        path: '/printer',
        builder: (context, state) => const BluetoothPrinterPage(),
      ),
    ],
  );
});

final routerNotifierProvider = Provider<RouterNotifier>((ref) {
  return RouterNotifier(ref);
});

class RouterNotifier extends ChangeNotifier {
  final Ref _ref;

  RouterNotifier(this._ref) {
    _ref.listen(authProvider, (previous, next) {
      if (previous?.isAuthenticated != next.isAuthenticated ||
          previous?.isLoading != next.isLoading) {
        notifyListeners();
      }
    });
  }

  String? redirect(BuildContext context, GoRouterState state) {
    final authState = _ref.read(authProvider);
    final currentPath = state.matchedLocation;
    final isLoggingIn = currentPath == '/login';
    final isWelcome = currentPath == '/welcome';
    final isSettings = currentPath.startsWith('/settings');
    final isPrinter = currentPath == '/printer';
    final isOrderDetail = currentPath == '/order-detail';
    final isTables = currentPath == '/tables';

    debugPrint(
      'Router: Redirect check - Auth: ${authState.isAuthenticated}, Loading: ${authState.isLoading}, Current: $currentPath',
    );

    // If we are checking initial auth, don't redirect yet
    if (authState.isLoading && authState.accessToken == null) {
      return null;
    }

    if (!authState.isAuthenticated) {
      if (isOrderDetail) {
        return '/';
      }

      if (currentPath == '/' ||
          isWelcome ||
          isLoggingIn ||
          isSettings ||
          isPrinter ||
          isTables) {
        return null;
      }

      return '/';
    }

    // If authenticated but on auth pages, go to home
    if (isWelcome || isLoggingIn) {
      return '/';
    }

    return null;
  }
}

class MerchantApp extends ConsumerWidget {
  const MerchantApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Keep background managers alive
    ref.watch(workingStatusSyncManagerProvider);
    ref.watch(deviceSyncManagerProvider);
    ref.watch(notificationPermissionManagerProvider);
    ref.watch(orderPollerManagerProvider);
    ref.watch(pendingOrderAlertDrainManagerProvider);

    final router = ref.watch(routerProvider);
    final authState = ref.watch(authProvider);
    final appTitle =
        (authState.merchantName != null &&
            authState.merchantName!.trim().isNotEmpty)
        ? authState.merchantName!.trim()
        : '商户工作台';

    return MaterialApp.router(
      title: appTitle,
      theme: AppTheme.lightTheme,
      routerConfig: router,
      debugShowCheckedModeBanner: false,
      builder: (context, child) {
        // Show a global loading overlay instead of replacing the entire app
        return Stack(
          children: [
            child ?? const SizedBox.shrink(),
            if (authState.isLoading)
              Container(
                color: Colors.black26,
                child: const Center(child: CircularProgressIndicator()),
              ),
          ],
        );
      },
    );
  }
}
