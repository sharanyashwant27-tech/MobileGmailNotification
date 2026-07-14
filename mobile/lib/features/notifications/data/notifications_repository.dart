import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/models.dart';

class NotificationsRepository {
  final Dio _dio;

  NotificationsRepository(this._dio);

  Future<List<NotificationItem>> list({int limit = 50, int offset = 0}) async {
    try {
      final res = await _dio.get(
        '/notifications',
        queryParameters: {'limit': limit, 'offset': offset},
      );
      final data = res.data['data'] as List? ?? [];
      return data
          .map((e) => NotificationItem.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<void> markRead(String id) async {
    try {
      await _dio.post('/notifications/$id/read');
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<void> markAllRead() async {
    try {
      await _dio.post('/notifications/read-all');
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<int> unreadCount() async {
    try {
      final res = await _dio.get('/notifications/unread-count');
      return (res.data['data']['unread'] as num?)?.toInt() ?? 0;
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<NotificationSettings> getSettings() async {
    try {
      final res = await _dio.get('/settings/notifications');
      return NotificationSettings.fromJson(
        res.data['data'] as Map<String, dynamic>,
      );
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<NotificationSettings> updateSettings(NotificationSettings s) async {
    try {
      final res = await _dio.put('/settings/notifications', data: s.toJson());
      return NotificationSettings.fromJson(
        res.data['data'] as Map<String, dynamic>,
      );
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<void> registerDevice(String token, {String platform = 'android'}) async {
    try {
      await _dio.post('/devices', data: {
        'token': token,
        'platform': platform,
      });
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }
}
