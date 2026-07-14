class User {
  final String id;
  final String email;
  final String displayName;
  final bool darkMode;

  const User({
    required this.id,
    required this.email,
    required this.displayName,
    required this.darkMode,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: json['id'] as String,
        email: json['email'] as String,
        displayName: (json['display_name'] as String?) ?? '',
        darkMode: (json['dark_mode'] as bool?) ?? false,
      );
}

class AuthSession {
  final User user;
  final String accessToken;
  final String refreshToken;

  const AuthSession({
    required this.user,
    required this.accessToken,
    required this.refreshToken,
  });

  factory AuthSession.fromJson(Map<String, dynamic> json) => AuthSession(
        user: User.fromJson(json['user'] as Map<String, dynamic>),
        accessToken: json['access_token'] as String,
        refreshToken: json['refresh_token'] as String,
      );
}
