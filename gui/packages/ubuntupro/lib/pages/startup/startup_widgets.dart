import 'package:flutter/material.dart';

import '../../constants.dart';
import '../widgets/page_widgets.dart';
import '../widgets/status_bar.dart';

/// Builds a centered-column containg [bottom] and a [Text] widget containing
/// [message] or an empty string.
class StatusColumn extends StatelessWidget {
  const StatusColumn({super.key, this.top, this.message, this.bottom});
  final String? message;
  final Widget? top, bottom;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(kDefaultMargin),
      child: Center(
        child: SizedBox(
          width: 400,
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (top != null) ...[
                top!,
                const SizedBox(height: kDefaultMargin / 4),
              ],
              if (message != null) ...[
                Text(message!),
                const SizedBox(height: kDefaultMargin / 4),
              ],
              if (bottom != null) bottom!,
            ],
          ),
        ),
      ),
    );
  }
}

/// Displays a linear progress indicator at the top and the [statusMessage]
/// in the bottom, while hiding the title bar, so the window remains temporarily
/// unclosable
class StartupInProgressWidget extends StatelessWidget {
  const StartupInProgressWidget(this.message, {super.key});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Pro4WSLPage(
      body: StatusColumn(
        message: message,
        bottom: const LinearProgressIndicator(),
      ),
    );
  }
}

/// Displays an error icon followed by the [errorMessage] indicating a terminal
/// failure, i.e. no further action can be taken other than closing the app.
class StartupErrorWidget extends StatelessWidget {
  const StartupErrorWidget(this.message, {super.key});
  final String message;

  @override
  Widget build(BuildContext context) {
    return Pro4WSLPage(
      body: StatusColumn(
        top: const Icon(Icons.error_outline, size: 64),
        message: message,
      ),
      statusBar: const StatusBar(showAgentStatus: false),
    );
  }
}
