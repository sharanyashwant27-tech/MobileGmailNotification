class GmailAccount {
  final String id;
  final String email;
  final bool isActive;
  final bool notificationsOn;
  final DateTime? watchExpiration;
  final DateTime? lastSyncedAt;

  const GmailAccount({
    required this.id,
    required this.email,
    required this.isActive,
    required this.notificationsOn,
    this.watchExpiration,
    this.lastSyncedAt,
  });

  factory GmailAccount.fromJson(Map<String, dynamic> json) => GmailAccount(
        id: json['id'] as String,
        email: json['email'] as String,
        isActive: (json['is_active'] as bool?) ?? true,
        notificationsOn: (json['notifications_on'] as bool?) ?? true,
        watchExpiration: json['watch_expiration'] != null
            ? DateTime.tryParse(json['watch_expiration'] as String)
            : null,
        lastSyncedAt: json['last_synced_at'] != null
            ? DateTime.tryParse(json['last_synced_at'] as String)
            : null,
      );
}
