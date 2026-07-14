import 'package:dio/dio.dart';

import '../../../core/network/api_client.dart';
import '../domain/gmail_account.dart';

class AccountsRepository {
  final Dio _dio;

  AccountsRepository(this._dio);

  Future<List<GmailAccount>> list() async {
    try {
      final res = await _dio.get('/gmail/accounts');
      final data = res.data['data'] as List? ?? [];
      return data
          .map((e) => GmailAccount.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<String> beginLink() async {
    try {
      final res = await _dio.post('/gmail/accounts/link');
      return res.data['data']['authorization_url'] as String;
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<GmailAccount> setNotifications(String id, bool enabled) async {
    try {
      final res = await _dio.patch(
        '/gmail/accounts/$id/notifications',
        data: {'enabled': enabled},
      );
      return GmailAccount.fromJson(res.data['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<void> unlink(String id) async {
    try {
      await _dio.delete('/gmail/accounts/$id');
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }
}
