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
import 'package:yaru_widgets/yaru_widgets.dart';

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
            )
          : null,
      body: body,
      persistentFooterButtons: <Widget>[statusBar ?? const StatusBar()],
    );
  }
}

class ColumnLandingPage extends StatelessWidget {
  const ColumnLandingPage({
    super.key,
    required this.leftChildren,
    required this.children,
    this.onNext,
    this.onSkip,
    this.onBack,
    this.svgAsset = 'assets/Ubuntu-tag.svg',
    this.title = 'Landscape',
  });

  final List<Widget> leftChildren;
  final List<Widget> children;
  final String svgAsset;
  final String title;

  final void Function()? onNext;
  final void Function()? onSkip;
  final void Function()? onBack;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Pro4WSLPage(
      body: Stack(
        fit: StackFit.expand,
        children: [
          Positioned.fill(
            child: Image.asset(
              'assets/05_suru2_dark_2K.jpg',
              fit: BoxFit.fill,
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(32.0),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                IntrinsicHeight(
                  child: Row(
                    children: [
                      // Left column "header"
                      Expanded(
                        flex: 5,
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
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
                                    style: Theme.of(context)
                                        .textTheme
                                        .displaySmall
                                        ?.copyWith(fontWeight: FontWeight.w100),
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(
                              height: 24,
                            ),
                            ...leftChildren,
                          ],
                        ),
                      ),
                      // Divider
                      const Expanded(
                        flex: 1,
                        child: VerticalDivider(
                          thickness: 0.2,
                          color: Colors.white,
                        ),
                      ),
                      // Right column content
                      Expanded(
                        flex: 6,
                        child: Column(
                          children: [...children],
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(
                  height: 16.0,
                ),
                // Navigation buttons
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    OutlinedButton(
                      onPressed: onBack,
                      child: Text(lang.buttonBack),
                    ),
                    Row(
                      children: [
                        FilledButton(
                          onPressed: onSkip,
                          child: Text(lang.buttonSkip),
                        ),
                        const SizedBox(
                          width: 16.0,
                        ),
                        FilledButton(
                          onPressed: onNext,
                          style: Theme.of(context)
                              .filledButtonTheme
                              .style
                              ?.copyWith(
                                backgroundColor: MaterialStatePropertyAll(
                                  YaruColors.dark.success,
                                ),
                              ),
                          child: Text(lang.buttonNext),
                        ),
                      ],
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// A more stylized page that mimics the design of the https://ubuntu.com/pro
// landing page, with a dark background and an [svgAsset] logo followed by
// a title with some opacity, rendering the [children] in a column layout.
class DarkStyledLandingPage extends StatelessWidget {
  const DarkStyledLandingPage({
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
  // TODO: Remove those getters once we have a background image suitable for the light mode theme.
  static ThemeData get _data => yaruDark;
  static TextTheme get textTheme => _data.textTheme;

  @override
  Widget build(BuildContext context) {
    return Pro4WSLPage(
      body: Stack(
        fit: StackFit.expand,
        children: [
          Positioned.fill(
            child: Image.asset(
              'assets/05_suru2_dark_2K.jpg',
              fit: BoxFit.fill,
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(48.0),
            child: centered
                ? Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(maxWidth: 480.0),
                      child: _PageContent(
                        svgAsset: svgAsset,
                        title: title,
                        data: _data,
                        centered: true,
                        children: children,
                      ),
                    ),
                  )
                : _PageContent(
                    svgAsset: svgAsset,
                    title: title,
                    data: _data,
                    children: children,
                  ),
          ),
        ],
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
          height: 24,
        ),
        ...children,
      ],
    );
  }
}
