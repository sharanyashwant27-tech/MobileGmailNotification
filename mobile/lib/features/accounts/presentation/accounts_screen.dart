import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/network/api_client.dart';
import '../data/accounts_repository.dart';
import '../domain/gmail_account.dart';

final accountsRepositoryProvider = Provider(
  (ref) => AccountsRepository(ref.watch(dioProvider)),
);

final accountsProvider =
    AsyncNotifierProvider<AccountsNotifier, List<GmailAccount>>(
  AccountsNotifier.new,
);

class AccountsNotifier extends AsyncNotifier<List<GmailAccount>> {
  @override
  Future<List<GmailAccount>> build() {
    return ref.read(accountsRepositoryProvider).list();
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(accountsRepositoryProvider).list(),
    );
  }

  Future<void> linkAccount() async {
    final url = await ref.read(accountsRepositoryProvider).beginLink();
    final uri = Uri.parse(url);
    if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
      throw ApiException('Could not open Google authorization');
    }
  }

  Future<void> toggleNotifications(String id, bool enabled) async {
    await ref.read(accountsRepositoryProvider).setNotifications(id, enabled);
    await refresh();
  }

  Future<void> unlink(String id) async {
    await ref.read(accountsRepositoryProvider).unlink(id);
    await refresh();
  }
}

class AccountsScreen extends ConsumerWidget {
  const AccountsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accounts = ref.watch(accountsProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Gmail accounts'),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: () => ref.read(accountsProvider.notifier).refresh(),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () async {
          try {
            await ref.read(accountsProvider.notifier).linkAccount();
            if (context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text(
                    'Complete Google sign-in in the browser, then tap refresh.',
                  ),
                ),
              );
            }
          } catch (e) {
            if (context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text(e.toString())),
              );
            }
          }
        },
        icon: const Icon(Icons.add),
        label: const Text('Connect Gmail'),
      ),
      body: accounts.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text(e.toString())),
        data: (list) {
          if (list.isEmpty) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: Text(
                  'No Gmail accounts linked yet.\nConnect one with Google OAuth — we never ask for your Gmail password.',
                  textAlign: TextAlign.center,
                ),
              ),
            );
          }
          return ListView.separated(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 88),
            itemCount: list.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (context, i) {
              final a = list[i];
              return Card(
                child: ListTile(
                  leading: CircleAvatar(
                    backgroundColor: Theme.of(context).colorScheme.primaryContainer,
                    child: Text(
                      a.email.isNotEmpty ? a.email[0].toUpperCase() : '?',
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.onPrimaryContainer,
                      ),
                    ),
                  ),
                  title: Text(a.email),
                  subtitle: Text(
                    a.notificationsOn ? 'Notifications on' : 'Notifications off',
                  ),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Switch(
                        value: a.notificationsOn,
                        onChanged: (v) => ref
                            .read(accountsProvider.notifier)
                            .toggleNotifications(a.id, v),
                      ),
                      IconButton(
                        tooltip: 'Unlink',
                        icon: const Icon(Icons.link_off),
                        onPressed: () async {
                          final ok = await showDialog<bool>(
                            context: context,
                            builder: (ctx) => AlertDialog(
                              title: const Text('Unlink account?'),
                              content: Text('Remove ${a.email} and revoke watch.'),
                              actions: [
                                TextButton(
                                  onPressed: () => Navigator.pop(ctx, false),
                                  child: const Text('Cancel'),
                                ),
                                FilledButton(
                                  onPressed: () => Navigator.pop(ctx, true),
                                  child: const Text('Unlink'),
                                ),
                              ],
                            ),
                          );
                          if (ok == true) {
                            await ref.read(accountsProvider.notifier).unlink(a.id);
                          }
                        },
                      ),
                    ],
                  ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
