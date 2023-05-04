import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:yaru/yaru.dart';

import 'constants.dart';
import 'pages/enter_token/enter_token_page.dart';

class Pro4WindowsApp extends StatelessWidget {
  const Pro4WindowsApp({super.key});

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) => MaterialApp(
        title: kAppName,
        theme: yaru.theme,
        darkTheme: yaru.darkTheme,
        debugShowCheckedModeBanner: false,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        onGenerateTitle: (context) => AppLocalizations.of(context).appTitle,
        routes: const {
          '/': EnterProTokenPage.create,
        },
      ),
    );
  }
}
