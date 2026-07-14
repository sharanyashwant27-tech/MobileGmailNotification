import 'package:flutter_test/flutter_test.dart';
import 'package:gmail_notification/features/notifications/domain/models.dart';

void main() {
  test('NotificationSettings round-trips JSON', () {
    final s = NotificationSettings(
      enabled: true,
      quietHoursEnabled: true,
      quietHoursStart: '22:00',
      quietHoursEnd: '07:00',
      onlyPrimary: true,
      includeSpam: false,
      keywordFilter: 'invoice',
      senderAllowlist: '@acme.com',
    );
    final decoded = NotificationSettings.fromJson(s.toJson());
    expect(decoded.enabled, isTrue);
    expect(decoded.keywordFilter, 'invoice');
    expect(decoded.quietHoursStart, '22:00');
  });

  test('NotificationItem parses API payload', () {
    final item = NotificationItem.fromJson({
      'id': '11111111-1111-1111-1111-111111111111',
      'gmail_account_id': '22222222-2222-2222-2222-222222222222',
      'message_id': 'msg-1',
      'from_address': 'Alice <alice@example.com>',
      'subject': 'Hello',
      'snippet': 'Hi there',
      'received_at': '2026-07-14T10:00:00Z',
      'is_read': false,
    });
    expect(item.subject, 'Hello');
    expect(item.isRead, isFalse);
  });
}
