import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_test/flutter_test.dart';

extension L10nTester on WidgetTester {
  /// Returns the AppLocalizations object associated with the [BuildContext] of
  /// the first [Page] widget found by type.
  /// An assertion error will be thrown if the [Page] widget is not in the tree.
  AppLocalizations l10n<Page>() {
    final matcher = find.byType(Page);
    expect(matcher, findsWidgets);
    final view = element(matcher);
    return AppLocalizations.of(view);
  }
}
