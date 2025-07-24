import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wizard_router/wizard_router.dart';

import '/constants.dart';
import '/l10n/app_localizations.dart';
import '../landscape/landscape_model.dart';
import '../widgets/navigation_row.dart';
import '../widgets/page_widgets.dart';
import '../widgets/radio_tile.dart';

enum SkipEnum {
  skip,
  register,
}

class LandscapeSkipPage extends StatefulWidget {
  const LandscapeSkipPage({super.key});

  @override
  State<LandscapeSkipPage> createState() => _LandscapeSkipPageState();
}

class _LandscapeSkipPageState extends State<LandscapeSkipPage> {
  SkipEnum groupValue = SkipEnum.skip;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final wizard = Wizard.of(context);

    return ColumnPage(
      svgAsset: 'assets/Landscape-tag.svg',
      title: kLandscapeTitle,
      left: [
        MarkdownBody(
          data: lang.landscapeHeading(
            '[${lang.learnMore}](${LandscapeModel.landscapeURI})',
          ),
          onTapLink: (_, href, __) => launchUrl(LandscapeModel.landscapeURI),
        ),
      ],
      right: [
        RadioTile(
          value: SkipEnum.skip,
          title: lang.landscapeSkip,
          subtitle: lang.landscapeSkipDescription,
          groupValue: groupValue,
          onChanged: (v) => setState(() {
            groupValue = v!;
          }),
        ),
        const SizedBox(height: 16),
        RadioTile(
          value: SkipEnum.register,
          title: lang.landscapeSkipRegister,
          groupValue: groupValue,
          onChanged: (v) => setState(() {
            groupValue = v!;
          }),
        ),
      ],
      navigationRow: NavigationRow(
        onBack: wizard.back,
        onNext: () => wizard.next(arguments: groupValue),
        nextIsAction: false,
      ),
    );
  }
}
