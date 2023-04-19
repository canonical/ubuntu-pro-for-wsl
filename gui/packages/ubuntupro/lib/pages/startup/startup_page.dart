import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';

import '../../constants.dart';
import '../../core/agent_api_client.dart';
import '../widgets/page_widgets.dart';
import 'agent_monitor.dart';

import 'startup_model.dart';

part 'startup_widgets.dart';

/// A widget that decouples the instantiation of a [StartupModel] and its
/// consumer [StartupAnimatedChild] while offering the caller the [onClient] callback to
/// be executed when/if the [AgentApiClient] is made available by the view-model.
class StartupPage extends StatefulWidget {
  const StartupPage({
    super.key,
    required this.launcher,
    required this.nextRoute,
    required this.clientFactory,
    required this.onClient,
  });
  final AgentLauncher launcher;
  final ApiClientFactory clientFactory;
  final String nextRoute;
  final void Function(AgentApiClient) onClient;

  @override
  State<StartupPage> createState() => _StartupPageState();
}

class _StartupPageState extends State<StartupPage> {
  late AgentStartupMonitor monitor;
  @override
  void initState() {
    super.initState();
    monitor = AgentStartupMonitor(
      appName: kAppName,
      addrFileName: kAddrFileName,
      agentLauncher: widget.launcher,
      clientFactory: widget.clientFactory,
      onClient: widget.onClient,
    );
  }

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider<StartupModel>(
      create: (context) {
        final model = StartupModel(monitor);
        model.init();
        return model;
      },
      child: StartupAnimatedChild(nextRoute: widget.nextRoute),
    );
  }
}

/// A page that reports the background agent startup statuses by listening to a
/// [StartupModel] provided by the parent widget tree and transitions smoothly
/// to the predefined [nextRoute] once the agent is found responsive.
class StartupAnimatedChild extends StatefulWidget {
  /// The route where to transition to on success.
  final String nextRoute;

  const StartupAnimatedChild({super.key, required this.nextRoute});

  @override
  State<StartupAnimatedChild> createState() => _StartupAnimatedChildState();
}

class _StartupAnimatedChildState extends State<StartupAnimatedChild> {
  @override
  void initState() {
    super.initState();
    final model = context.read<StartupModel>();
    model.addListener(() {
      if (model.view == ViewState.ok) {
        Navigator.of(context).pushReplacementNamed(widget.nextRoute);
      }
    });
  }

  Widget buildChild(ViewState view, String message) {
    switch (view) {
      case ViewState.inProgress:
        return _StartupInProgressWidget(message);

      case ViewState.ok:
        return const SizedBox.shrink();

      case ViewState.retry:
        return _StartupRetryWidget(
          message: message,
          retry: OutlinedButton(
            onPressed: () => context.read<StartupModel>().resetAgent(),
            child: Text(AppLocalizations.of(context).agentRetryButton),
          ),
        );

      case ViewState.crash:
        return _StartupErrorWidget(message);
    }
  }

  @override
  Widget build(BuildContext context) {
    final model = context.watch<StartupModel>();
    final lang = AppLocalizations.of(context);

    return Material(
      child: AnimatedSwitcher(
        switchInCurve: Curves.easeInExpo,
        switchOutCurve: Curves.easeOutExpo,
        duration: kQuickAnimationDuration,
        child: buildChild(model.view, model.message(lang)),
      ),
    );
  }
}
