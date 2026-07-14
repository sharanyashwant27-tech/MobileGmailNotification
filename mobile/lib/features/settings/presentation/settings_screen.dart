import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../auth/presentation/auth_controller.dart';
import '../../notifications/data/notifications_repository.dart';
import '../../notifications/domain/models.dart';
import '../../notifications/presentation/notifications_screen.dart';

final darkModeProvider =
    NotifierProvider<DarkModeNotifier, bool>(DarkModeNotifier.new);

class DarkModeNotifier extends Notifier<bool> {
  static const _key = 'dark_mode';

  @override
  bool build() {
    Future(() async {
      final prefs = await SharedPreferences.getInstance();
      final local = prefs.getBool(_key);
      final user = ref.read(authControllerProvider).user;
      state = local ?? user?.darkMode ?? false;
    });
    return false;
  }

  Future<void> setDarkMode(bool value) async {
    state = value;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_key, value);
    final auth = ref.read(authControllerProvider);
    if (auth.isAuthenticated) {
      await auth.setDarkMode(value);
    }
  }
}

final notificationSettingsProvider =
    AsyncNotifierProvider<SettingsNotifier, NotificationSettings>(
  SettingsNotifier.new,
);

class SettingsNotifier extends AsyncNotifier<NotificationSettings> {
  @override
  Future<NotificationSettings> build() {
    return ref.read(notificationsRepositoryProvider).getSettings();
  }

  Future<void> save(NotificationSettings settings) async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(notificationsRepositoryProvider).updateSettings(settings),
    );
  }
}

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dark = ref.watch(darkModeProvider);
    final settings = ref.watch(notificationSettingsProvider);
    final user = ref.watch(authControllerProvider).user;

    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (user != null) ...[
            ListTile(
              contentPadding: EdgeInsets.zero,
              leading: CircleAvatar(
                child: Text(
                  user.displayName.isNotEmpty
                      ? user.displayName[0].toUpperCase()
                      : user.email[0].toUpperCase(),
                ),
              ),
              title: Text(
                user.displayName.isEmpty ? user.email : user.displayName,
              ),
              subtitle: Text(user.email),
            ),
            const Divider(height: 32),
          ],
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            title: const Text('Dark mode'),
            subtitle: const Text('Use Material dark theme'),
            value: dark,
            onChanged: (v) => ref.read(darkModeProvider.notifier).setDarkMode(v),
          ),
          const SizedBox(height: 8),
          Text('Notification filters', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          settings.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (e, _) => Text(e.toString()),
            data: (s) => _SettingsForm(
              initial: s,
              onSave: (next) async {
                await ref.read(notificationSettingsProvider.notifier).save(next);
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Settings saved')),
                  );
                }
              },
            ),
          ),
          const Divider(height: 40),
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: const Icon(Icons.logout),
            title: const Text('Sign out'),
            onTap: () => ref.read(authControllerProvider).logout(),
          ),
          const SizedBox(height: 8),
          Text(
            'Gmail access uses OAuth tokens only. This app never requests or stores Gmail passwords.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _SettingsForm extends StatefulWidget {
  final NotificationSettings initial;
  final Future<void> Function(NotificationSettings) onSave;

  const _SettingsForm({required this.initial, required this.onSave});

  @override
  State<_SettingsForm> createState() => _SettingsFormState();
}

class _SettingsFormState extends State<_SettingsForm> {
  late NotificationSettings _s;
  late final TextEditingController _keyword;
  late final TextEditingController _allow;

  @override
  void initState() {
    super.initState();
    _s = widget.initial;
    _keyword = TextEditingController(text: _s.keywordFilter);
    _allow = TextEditingController(text: _s.senderAllowlist);
  }

  @override
  void dispose() {
    _keyword.dispose();
    _allow.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Push notifications'),
          value: _s.enabled,
          onChanged: (v) => setState(() => _s = _s.copyWith(enabled: v)),
        ),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Primary inbox only'),
          value: _s.onlyPrimary,
          onChanged: (v) => setState(() => _s = _s.copyWith(onlyPrimary: v)),
        ),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Include spam'),
          value: _s.includeSpam,
          onChanged: (v) => setState(() => _s = _s.copyWith(includeSpam: v)),
        ),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          title: const Text('Quiet hours'),
          subtitle: Text('${_s.quietHoursStart} – ${_s.quietHoursEnd}'),
          value: _s.quietHoursEnabled,
          onChanged: (v) =>
              setState(() => _s = _s.copyWith(quietHoursEnabled: v)),
        ),
        TextField(
          controller: _keyword,
          decoration: const InputDecoration(
            labelText: 'Keyword filter (optional)',
            hintText: 'invoice',
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _allow,
          decoration: const InputDecoration(
            labelText: 'Sender allowlist (comma-separated)',
            hintText: '@company.com, boss@',
          ),
        ),
        const SizedBox(height: 16),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton(
            onPressed: () => widget.onSave(
              _s.copyWith(
                keywordFilter: _keyword.text.trim(),
                senderAllowlist: _allow.text.trim(),
              ),
            ),
            child: const Text('Save filters'),
          ),
        ),
      ],
    );
  }
}
