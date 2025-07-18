import 'package:flutter/material.dart';
import 'package:ubuntupro/l10n/app_localizations.dart';
import 'package:provider/provider.dart';
import 'package:wizard_router/wizard_router.dart';

import '/core/agent_api_client.dart';
import '/core/agent_monitor.dart';
import 'startup_model.dart';
import 'startup_widgets.dart';

/// A widget that decouples the instantiation of a [StartupModel] and its
/// consumer [StartupAnimatedChild] while offering the caller the [onClient] callback to
/// be executed when/if the [AgentApiClient] is made available by the view-model.
class StartupPage extends StatelessWidget {
  const StartupPage({super.key});

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider<StartupModel>(
      create: (context) {
        final monitor = context.read<AgentStartupMonitor>();
        final model = StartupModel(monitor);
        return model;
      },
      child: const StartupAnimatedChild(),
    );
  }
}

/// A page that reports the background agent startup statuses by listening to a
/// [StartupModel] provided by the parent widget tree and transitions smoothly
/// to the Wizard's next route once the agent is found responsive.
class StartupAnimatedChild extends StatefulWidget {
  const StartupAnimatedChild({super.key});

  @override
  State<StartupAnimatedChild> createState() => _StartupAnimatedChildState();
}

class _StartupAnimatedChildState extends State<StartupAnimatedChild> {
  @override
  void initState() {
    super.initState();
    final model = context.read<StartupModel>();
    model.init();
    model.addListener(() async {
      if (model.view == ViewState.ok) {
        await Wizard.of(context).replace();
      }
      if (model.view == ViewState.retry) {
        await model.resetAgent();
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
