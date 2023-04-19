part of 'startup_page.dart';

/// Builds a centered-column containg [bottom] and a [Text] widget containing
/// [message] or an empty string.
class _StatusColumn extends StatelessWidget {
  const _StatusColumn({
    this.top,
    this.message,
    this.bottom,
  });
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
class _StartupInProgressWidget extends StatelessWidget {
  const _StartupInProgressWidget(this.message);

  final String message;

  @override
  Widget build(BuildContext context) {
    return Pro4WindowsPage(
      body: _StatusColumn(
        message: message,
        bottom: const LinearProgressIndicator(),
      ),
      showTitleBar: false,
    );
  }
}

/// Displays an error icon followed by the [errorMessage] indicating a terminal
/// failure, i.e. no further action can be taken other than closing the app.
class _StartupErrorWidget extends StatelessWidget {
  const _StartupErrorWidget(this.message);
  final String message;

  @override
  Widget build(BuildContext context) {
    return Pro4WindowsPage(
      body: _StatusColumn(
        top: const Icon(Icons.error_outline, size: 64),
        message: message,
      ),
    );
  }
}

/// Displays an error icon followed by the [errorMessage] and a button allowing
/// users to manually request a reset/retry operation.
class _StartupRetryWidget extends StatelessWidget {
  const _StartupRetryWidget({required this.message, required this.retry});
  final String message;
  final Widget retry;

  @override
  Widget build(BuildContext context) {
    return Pro4WindowsPage(
      body: _StatusColumn(
        top: const Icon(Icons.error_outline, size: 64),
        message: message,
        bottom: retry,
      ),
    );
  }
}
