import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';
import '../../../core/storage/token_storage.dart';
import '../data/auth_repository.dart';
import '../domain/user.dart';

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(ref.watch(dioProvider));
});

final authControllerProvider =
    ChangeNotifierProvider<AuthController>((ref) {
  return AuthController(
    ref.watch(authRepositoryProvider),
    ref.watch(tokenStorageProvider),
  )..bootstrap();
});

class AuthController extends ChangeNotifier {
  final AuthRepository _repo;
  final TokenStorage _storage;

  User? user;
  bool bootstrapping = true;
  String? error;

  AuthController(this._repo, this._storage);

  bool get isAuthenticated => user != null;

  Future<void> bootstrap() async {
    bootstrapping = true;
    notifyListeners();
    try {
      final token = await _storage.accessToken;
      if (token != null && token.isNotEmpty) {
        user = await _repo.me();
      }
    } catch (_) {
      await _storage.clear();
      user = null;
    } finally {
      bootstrapping = false;
      notifyListeners();
    }
  }

  Future<bool> login(String email, String password) async {
    error = null;
    notifyListeners();
    try {
      final session = await _repo.login(email: email, password: password);
      await _storage.saveTokens(
        accessToken: session.accessToken,
        refreshToken: session.refreshToken,
      );
      user = session.user;
      notifyListeners();
      return true;
    } catch (e) {
      error = e.toString();
      notifyListeners();
      return false;
    }
  }

  Future<bool> register(String email, String password, String name) async {
    error = null;
    notifyListeners();
    try {
      final session = await _repo.register(
        email: email,
        password: password,
        displayName: name,
      );
      await _storage.saveTokens(
        accessToken: session.accessToken,
        refreshToken: session.refreshToken,
      );
      user = session.user;
      notifyListeners();
      return true;
    } catch (e) {
      error = e.toString();
      notifyListeners();
      return false;
    }
  }

  Future<void> logout() async {
    await _repo.logout();
    await _storage.clear();
    user = null;
    notifyListeners();
  }

  Future<void> setDarkMode(bool value) async {
    if (user == null) return;
    user = await _repo.updateProfile(darkMode: value);
    notifyListeners();
  }
}
