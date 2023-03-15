import 'package:flutter/material.dart';
import 'package:yaru/yaru.dart';

import 'pages/enter_token/enter_token_page.dart';

class Pro4WindowsApp extends StatelessWidget {
  const Pro4WindowsApp({super.key});

  @override
  Widget build(BuildContext context) {
    return YaruTheme(
      builder: (context, yaru, child) => MaterialApp(
        title: 'Yaru Flutter Windows',
        theme: yaru.theme,
        darkTheme: yaru.darkTheme,
        debugShowCheckedModeBanner: false,
        supportedLocales: {
          const Locale('en'), // make sure 'en' comes first
          // TODO: Setup l10n
          // ...List.of(AppLocalizations.supportedLocales)
          //   ..remove(const Locale('en')),
        },
        onGenerateTitle: (context) => 'Yaru Flutter Windows',
        routes: const {
          '/': EnterProTokenPage.create,
        },
      ),
    );
  }
}
