import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../auth/presentation/auth_controller.dart';
import '../data/notifications_repository.dart';
import '../presentation/notifications_screen.dart';

@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  // History is persisted server-side; foreground UI refreshes on next open.
  debugPrint('FCM background: ${message.messageId}');
}

final fcmBootstrapProvider = Provider<void>((ref) {
  final auth = ref.watch(authControllerProvider);
  if (!auth.isAuthenticated) return;

  Future(() async {
    try {
      FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);
      final messaging = FirebaseMessaging.instance;
      await messaging.requestPermission();
      final token = await messaging.getToken();
      if (token != null) {
        await ref
            .read(notificationsRepositoryProvider)
            .registerDevice(token);
      }
      messaging.onTokenRefresh.listen((t) {
        ref.read(notificationsRepositoryProvider).registerDevice(t);
      });
      FirebaseMessaging.onMessage.listen((_) {
        ref.read(notificationsProvider.notifier).refresh();
      });
    } catch (e) {
      debugPrint('FCM bootstrap skipped: $e');
    }
  });
});
