/// Common UI elements to centralize some design decisions, enforce i18n and
/// avoid code repetition.
///
/// One such example is the usage of an app bar. More complex widgets and
/// pages are likely to have their own [Scaffold], which may require an [AppBar].
/// When using the [yaru_widgets](https://pub.dev/packages/yaru_widgets),
/// every page having a [Scaffold] and willing to show window controls will
/// have a build method like:
///
/// ```dart
/// return Scaffold(
///   final lang = AppLocalizations.of(context);
///   appBar: YaruWindowTitleBar(title: lang.appTitle),
/// ...
/// ```
///
/// Besides code repetition it's possible for different pages to show completely
/// different app bars, other than a [YaruWindowTitleBar], thus dstroying the
/// app consistency. Sticking to the [Pro4WSLPage] widget allows for a
/// consistent app/title bar with minimal code repetition. Should we adopt
/// something else other than the [YaruWindowTitleBar] in the future,
/// there will be less places to change.
library;

import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:yaru/yaru.dart';

import 'navigation_row.dart';
import 'status_bar.dart';

/// The simplest material page that covers most of the use cases in this app,
/// which may have a consistent title bar or no title bar at all.
class Pro4WSLPage extends StatelessWidget {
  // This should be updated to reflect the common use cases in this app.
  // Should we start using other Scaffold elements everywhere in the app,
  // such as the list of actions or the floating action button, we should
  // update this class to reflect that pattern.
  const Pro4WSLPage({
    super.key,
    required this.body,
    this.showTitleBar = true,
    this.statusBar,
  });

  /// Whether to show the window title bar or not. Defaults to true.
  final bool showTitleBar;

  /// The [Scaffold] body widget.
  final Widget body;

  /// The status bar widget to be shown at the bottom of the page.
  final Widget? statusBar;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    return Scaffold(
      appBar: showTitleBar
          ? YaruWindowTitleBar(
              title: Text(lang.appTitle),
              buttonPadding: EdgeInsets.zero,
            )
          : null,
      body: body,
      persistentFooterButtons: <Widget>[statusBar ?? const StatusBar()],
    );
  }
}

// A more stylized page that mimics the design of the https://ubuntu.com/pro
// landing page, with a dark background and an [svgAsset] logo followed by
// a title with some opacity, rendering the [children] in a column layout.
class LandingPage extends StatelessWidget {
  const LandingPage({
    super.key,
    required this.children,
    this.svgAsset = 'assets/Ubuntu-tag.svg',
    this.title = 'Ubuntu Pro',
    this.centered = false,
  });
  final List<Widget> children;
  final String svgAsset;
  final String title;
  final bool centered;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Pro4WSLPage(
      body: Padding(
        padding: const EdgeInsets.fromLTRB(32.0, 32.0, 32.0, 8.0),
        child: centered
            ? Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 480.0),
                  child: _PageContent(
                    svgAsset: svgAsset,
                    title: title,
                    data: theme,
                    centered: true,
                    children: children,
                  ),
                ),
              )
            : _PageContent(
                svgAsset: svgAsset,
                title: title,
                data: theme,
                children: children,
              ),
      ),
    );
  }
}

class _PageContent extends StatelessWidget {
  const _PageContent({
    required this.svgAsset,
    required this.title,
    required ThemeData data,
    required this.children,
    this.centered = false,
  }) : _data = data;

  final String svgAsset;
  final String title;
  final ThemeData _data;
  final List<Widget> children;
  final bool centered;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment:
          centered ? CrossAxisAlignment.center : CrossAxisAlignment.start,
      mainAxisAlignment:
          centered ? MainAxisAlignment.center : MainAxisAlignment.start,
      children: [
        RichText(
          text: TextSpan(
            children: [
              WidgetSpan(
                child: SvgPicture.asset(
                  svgAsset,
                  height: 70,
                ),
              ),
              const WidgetSpan(
                child: SizedBox(
                  width: 8,
                ),
              ),
              TextSpan(
                text: title,
                style: _data.textTheme.displaySmall
                    ?.copyWith(fontWeight: FontWeight.w100),
              ),
            ],
          ),
        ),
        const SizedBox(
          height: 12,
        ),
        ...children,
      ],
    );
  }
}

class CenteredPage extends StatelessWidget {
  const CenteredPage({
    super.key,
    required this.children,
    this.svgAsset = 'assets/Ubuntu-tag.svg',
    this.title = 'Ubuntu Pro',
    this.footer,
  });

  final List<Widget> children;
  final Widget? footer;
  final String svgAsset;
  final String title;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Pro4WSLPage(
      body: Padding(
        padding: const EdgeInsets.fromLTRB(32.0, 24.0, 32.0, 32.0),
        child: Column(
          children: [
            Expanded(
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 540.0),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        RichText(
                          text: TextSpan(
                            children: [
                              WidgetSpan(
                                child: SvgPicture.asset(
                                  svgAsset,
                                  height: 70,
                                ),
                              ),
                              const WidgetSpan(
                                child: SizedBox(
                                  width: 8,
                                ),
                              ),
                              TextSpan(
                                text: title,
                                style: theme.textTheme.displaySmall
                                    ?.copyWith(fontWeight: FontWeight.w100),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 16),
                        ...children,
                      ],
                    ),
                  ),
                ],
              ),
            ),
            if (footer != null) footer!,
          ],
        ),
      ),
    );
  }
}

/// Two-column, vertically centered page. The left column always contains the
/// svg image and title, with the left children below it. Both columns are equal
/// in width. Optionally, a [NavigationRow] may be provided that will span the
/// width below both columns.
class ColumnPage extends StatelessWidget {
  const ColumnPage({
    required this.left,
    required this.right,
    this.svgAsset = 'assets/Ubuntu-tag.svg',
    this.title = 'Ubuntu Pro',
    this.navigationRow,
    super.key,
  });

  final List<Widget> left;
  final List<Widget> right;
  final String svgAsset;
  final String title;
  final NavigationRow? navigationRow;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Pro4WSLPage(
      body: Padding(
        padding: const EdgeInsets.fromLTRB(32.0, 24.0, 32.0, 24.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Expanded(
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  // Left column
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        RichText(
                          text: TextSpan(
                            children: [
                              WidgetSpan(
                                child: SvgPicture.asset(
                                  svgAsset,
                                  height: 70,
                                ),
                              ),
                              const WidgetSpan(
                                child: SizedBox(
                                  width: 8,
                                ),
                              ),
                              TextSpan(
                                text: title,
                                style: theme.textTheme.displaySmall
                                    ?.copyWith(fontWeight: FontWeight.w100),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: 24),
                        ...left,
                      ],
                    ),
                  ),
                  // Spacer
                  const SizedBox(width: 32),
                  // Right column
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: right,
                    ),
                  ),
                ],
              ),
            ),
            if (navigationRow != null) navigationRow!,
          ],
        ),
      ),
    );
  }
}
