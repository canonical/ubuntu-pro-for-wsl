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
  });

  /// Whether to show the window title bar or not. Defaults to true.
  final bool showTitleBar;

  /// The [Scaffold] body widget.
  final Widget body;

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
  });
  final List<Widget> children;
  final String svgAsset;
  final String title;
  // TODO: Remove those getters once we have a background image suitable for the light mode theme.
  static ThemeData get _data => yaruDark;
  static TextTheme get textTheme => _data.textTheme;

  @override
  Widget build(BuildContext context) {
    return Pro4WSLPage(
      body: Stack(
        children: [
          Positioned.fill(
            child: Image.asset(
              'assets/05_suru2_dark_2K.jpg',
              fit: BoxFit.fill,
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(48.0),
            child: Column(
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
            ),
          ),
        ],
      ),
    );
  }
}
