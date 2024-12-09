import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wizard_router/wizard_router.dart';

import '../../constants.dart';
import '../../routes.dart';
import '../landscape/landscape_model.dart';
import '../widgets/navigation_row.dart';
import '../widgets/page_widgets.dart';
import '../widgets/radio_tile.dart';

enum _SkipEnum {
  skip,
  register,
}

class LandscapeSkipPage extends StatefulWidget {
  const LandscapeSkipPage({super.key});

  @override
  State<LandscapeSkipPage> createState() => _LandscapeSkipPageState();
}

class _LandscapeSkipPageState extends State<LandscapeSkipPage> {
  _SkipEnum groupValue = _SkipEnum.skip;

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
          value: _SkipEnum.skip,
          title: lang.landscapeSkip,
          subtitle: lang.landscapeSkipDescription,
          groupValue: groupValue,
          onChanged: (v) => setState(() {
            groupValue = v!;
          }),
        ),
        RadioTile(
          value: _SkipEnum.register,
          title: lang.landscapeSkipRegister,
          groupValue: groupValue,
          onChanged: (v) => setState(() {
            groupValue = v!;
          }),
        ),
      ],
      navigationRow: NavigationRow(
        onBack: wizard.back,
        onNext: () {
          switch (groupValue) {
            case _SkipEnum.skip:
              wizard.jump(Routes.subscriptionStatus);
            case _SkipEnum.register:
              wizard.next();
          }
        },
      ),
    );
  }
}
