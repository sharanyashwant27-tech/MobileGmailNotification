class NotificationItem {
  final String id;
  final String gmailAccountId;
  final String messageId;
  final String fromAddress;
  final String subject;
  final String snippet;
  final DateTime receivedAt;
  final bool isRead;

  const NotificationItem({
    required this.id,
    required this.gmailAccountId,
    required this.messageId,
    required this.fromAddress,
    required this.subject,
    required this.snippet,
    required this.receivedAt,
    required this.isRead,
  });

  factory NotificationItem.fromJson(Map<String, dynamic> json) =>
      NotificationItem(
        id: json['id'] as String,
        gmailAccountId: json['gmail_account_id'] as String,
        messageId: json['message_id'] as String,
        fromAddress: (json['from_address'] as String?) ?? '',
        subject: (json['subject'] as String?) ?? '',
        snippet: (json['snippet'] as String?) ?? '',
        receivedAt: DateTime.tryParse(json['received_at'] as String? ?? '') ??
            DateTime.now(),
        isRead: (json['is_read'] as bool?) ?? false,
      );
}

class NotificationSettings {
  final bool enabled;
  final bool quietHoursEnabled;
  final String quietHoursStart;
  final String quietHoursEnd;
  final bool onlyPrimary;
  final bool includeSpam;
  final String keywordFilter;
  final String senderAllowlist;

  const NotificationSettings({
    required this.enabled,
    required this.quietHoursEnabled,
    required this.quietHoursStart,
    required this.quietHoursEnd,
    required this.onlyPrimary,
    required this.includeSpam,
    required this.keywordFilter,
    required this.senderAllowlist,
  });

  factory NotificationSettings.fromJson(Map<String, dynamic> json) =>
      NotificationSettings(
        enabled: (json['enabled'] as bool?) ?? true,
        quietHoursEnabled: (json['quiet_hours_enabled'] as bool?) ?? false,
        quietHoursStart: (json['quiet_hours_start'] as String?) ?? '22:00',
        quietHoursEnd: (json['quiet_hours_end'] as String?) ?? '07:00',
        onlyPrimary: (json['only_primary'] as bool?) ?? true,
        includeSpam: (json['include_spam'] as bool?) ?? false,
        keywordFilter: (json['keyword_filter'] as String?) ?? '',
        senderAllowlist: (json['sender_allowlist'] as String?) ?? '',
      );

  Map<String, dynamic> toJson() => {
        'enabled': enabled,
        'quiet_hours_enabled': quietHoursEnabled,
        'quiet_hours_start': quietHoursStart,
        'quiet_hours_end': quietHoursEnd,
        'only_primary': onlyPrimary,
        'include_spam': includeSpam,
        'keyword_filter': keywordFilter,
        'sender_allowlist': senderAllowlist,
      };

  NotificationSettings copyWith({
    bool? enabled,
    bool? quietHoursEnabled,
    String? quietHoursStart,
    String? quietHoursEnd,
    bool? onlyPrimary,
    bool? includeSpam,
    String? keywordFilter,
    String? senderAllowlist,
  }) {
    return NotificationSettings(
      enabled: enabled ?? this.enabled,
      quietHoursEnabled: quietHoursEnabled ?? this.quietHoursEnabled,
      quietHoursStart: quietHoursStart ?? this.quietHoursStart,
      quietHoursEnd: quietHoursEnd ?? this.quietHoursEnd,
      onlyPrimary: onlyPrimary ?? this.onlyPrimary,
      includeSpam: includeSpam ?? this.includeSpam,
      keywordFilter: keywordFilter ?? this.keywordFilter,
      senderAllowlist: senderAllowlist ?? this.senderAllowlist,
    );
  }
}
