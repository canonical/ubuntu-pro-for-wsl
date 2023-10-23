import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:provider/provider.dart';

import '../../core/agent_api_client.dart';
import 'agent_monitor.dart';
import 'startup_model.dart';
import 'startup_widgets.dart';

/// A widget that decouples the instantiation of a [StartupModel] and its
/// consumer [StartupAnimatedChild] while offering the caller the [onClient] callback to
/// be executed when/if the [AgentApiClient] is made available by the view-model.
class StartupPage extends StatelessWidget {
  const StartupPage({
    super.key,
    required this.nextRoute,
  });

  final String nextRoute;

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider<StartupModel>(
      create: (context) {
        final monitor = context.read<AgentStartupMonitor>();
        final model = StartupModel(monitor);
        return model;
      },
      child: StartupAnimatedChild(nextRoute: nextRoute),
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
    model.init();
    model.addListener(() {
      if (model.view == ViewState.ok) {
        Navigator.of(context).pushReplacementNamed(widget.nextRoute);
      }
      if (model.view == ViewState.retry) {
        model.resetAgent();
      }
    });
  }

  Widget buildChild(ViewState view, String message) {
    switch (view) {
      case ViewState.inProgress:
      case ViewState.retry:
        return StartupInProgressWidget(message);

      case ViewState.ok:
        return const SizedBox.shrink();

      case ViewState.crash:
        return StartupErrorWidget(message);
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
        duration: kThemeAnimationDuration,
        child: buildChild(model.view, model.details.localize(lang)),
      ),
    );
  }
}

extension AgentStateL10n on AgentState {
  /// Allows representing the [AgentState] enum as a translatable String.
  String localize(AppLocalizations lang) {
    switch (this) {
      case AgentState.starting:
        return lang.agentStateStarting;
      case AgentState.pingNonResponsive:
        return lang.agentStatePingNonResponsive;
      case AgentState.invalid:
        return lang.agentStateInvalid;
      case AgentState.cannotStart:
        return lang.agentStateCannotStart;
      case AgentState.unknownEnv:
        return lang.agentStateUnknownEnv;
      case AgentState.querying:
        return lang.agentStateQuerying;
      case AgentState.unreachable:
        return lang.agentStateUnreachable;
      case AgentState.ok:
        // This state should not need translations.
        return '';
    }
  }
}
