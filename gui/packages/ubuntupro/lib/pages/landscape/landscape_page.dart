import 'dart:async';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/widgets.dart';
import 'package:yaru/yaru.dart';

import '/core/agent_api_client.dart';
import '/pages/widgets/page_widgets.dart';
import 'landscape_model.dart';

const _kHeight = 8.0;

/// Defines the overall structure of the Landscape configuration page and seggregates
/// the portions of the page that must rebuild at the relevant state changes.
class LandscapePage extends StatelessWidget {
  const LandscapePage({
    super.key,
    required this.onApplyConfig,
    required this.onBack,
    required this.onSkip,
  });

  /// Callable invoked when this page successfully applies the configuration.
  final void Function() onApplyConfig;

  /// Callable invoked when the user navigates back.
  final void Function() onBack;

  /// Callable invoked when the user skips this page.
  final void Function() onSkip;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final theme = Theme.of(context);
    final linkStyle = MarkdownStyleSheet.fromTheme(
      Theme.of(context).copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w100,
          ),
        ),
      ),
    ).copyWith(
      a: const TextStyle(
        decoration: TextDecoration.underline,
      ),
    );

    return LandingPage(
      svgAsset: 'assets/Landscape-tag.svg',
      title: 'Landscape',
      children: [
        // Only rebuilds if the value of model.landscapeURI changes (never in production)
        Selector<LandscapeModel, Uri>(
          selector: (_, model) => model.landscapeURI,
          builder: (context, uri, _) => MarkdownBody(
            data: lang.landscapeHeading('[Landscape]($uri)'),
            onTapLink: (_, href, __) => launchUrl(uri),
            styleSheet: linkStyle,
          ),
        ),

        // Main content: will rebuild whenever model notifies listeners, no filtering.
        Consumer<LandscapeModel>(
          builder: (context, model, _) => LandscapeConfigForm(model),
        ),
        const Spacer(),
        // Navigation buttons: only rebuild when the value of model.isComplete changes.
        Selector<LandscapeModel, bool>(
          selector: (_, model) => model.isComplete,
          builder: (context, isComplete, _) => _NavigationButtonRow(
            onBack: onBack,
            onSkip: onSkip,
            onNext: isComplete ? () => _tryApplyConfig(context) : null,
          ),
        ),
      ],
    );
  }

  Future<void> _tryApplyConfig(BuildContext context) async {
    final err = await context.read<LandscapeModel>().applyConfig();
    // Nothing else is safe to be done if the context is no longer mounted.
    assert(context.mounted);
    // The assertion is compiled away, so the linter will still complain if we don't check this in production.
    if (!context.mounted) {
      return;
    }
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(AppLocalizations.of(context).landscapeApplyError(err)),
        ),
      );
    } else {
      onApplyConfig();
    }
  }

  /// Creates a new Landscape page with its associated model connected to the Wizard
  static Widget create(BuildContext context, {bool isLate = false}) {
    final client = getService<AgentApiClient>();
    LandscapePage landscapePage;
    if (isLate) {
      landscapePage = LandscapePage(
        onApplyConfig: Wizard.of(context).back,
        onBack: Wizard.of(context).back,
        onSkip: Wizard.of(context).back,
      );
    } else {
      landscapePage = LandscapePage(
        onApplyConfig: Wizard.of(context).next,
        onBack: Wizard.of(context).back,
        onSkip: Wizard.of(context).next,
      );
    }

    return ChangeNotifierProvider<LandscapeModel>(
      create: (context) => LandscapeModel(client),
      child: landscapePage,
    );
  }
}

/// Defines the configuration form for the Landscape page, with special care for consistent keyboard navigation.
class LandscapeConfigForm extends StatelessWidget {
  const LandscapeConfigForm(this.model, {super.key});
  final LandscapeModel model;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2 * _kHeight),
      // The FocusTraversalGroup is necessary to keep the tab navigation order wed expect:
      // We ping-pong between the radio buttons and the form fields that belong to the selected radio button,
      // by assigning odd NumericFocusOrder() values to the radio buttons (on the left) and even values to the form fields,
      // while still skipping the invisible form fields.
      child: FocusTraversalGroup(
        policy: OrderedTraversalPolicy(),
        // Although IntrinsicHeight is an expensive widget, it is necessary to make the Row find a height constraint.
        // Otherwise the VerticalDivider will not be drawn.
        child: IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.start,
            children: [
              Flexible(
                child: Column(
                  children: [
                    if (model.isSaaSSupported)
                      FocusTraversalOrder(
                        order: const NumericFocusOrder(0),
                        child: _ConfigTypeRadio(
                          value: LandscapeConfigType.saas,
                          title: lang.landscapeQuickSetupSaas,
                          subtitle: lang.landscapeQuickSetupSaasHint,
                          groupValue: model.configType,
                          onChanged: model.setConfigType,
                        ),
                      ),
                    FocusTraversalOrder(
                      order: const NumericFocusOrder(2),
                      child: _ConfigTypeRadio(
                        value: LandscapeConfigType.selfHosted,
                        title: lang.landscapeQuickSetupSelfHosted,
                        subtitle: lang.landscapeQuickSetupSelfHostedHint,
                        groupValue: model.configType,
                        onChanged: model.setConfigType,
                      ),
                    ),
                    FocusTraversalOrder(
                      order: const NumericFocusOrder(4),
                      child: _ConfigTypeRadio(
                        value: LandscapeConfigType.custom,
                        title: lang.landscapeCustomSetup,
                        subtitle: lang.landscapeCustomSetupHint,
                        groupValue: model.configType,
                        onChanged: model.setConfigType,
                      ),
                    ),
                  ],
                ),
              ),
              const VerticalDivider(
                thickness: 2.0,
                width: 16.0,
              ),
              Flexible(
                // Thanks to IndexedStack, all three subforms exist, which prevents dismissing their states when the user
                // transitions between the config type options, but only one is shown at time.
                // We disable focusability for the invisible forms to prevent tabbing into them.
                child: IndexedStack(
                  index: model.configType.index,
                  children: [
                    FocusTraversalOrder(
                      order: const NumericFocusOrder(1),
                      child: FocusTraversalGroup(
                        descendantsAreFocusable:
                            model.configType == LandscapeConfigType.saas,
                        child: _SaasForm(model),
                      ),
                    ),
                    FocusTraversalOrder(
                      order: const NumericFocusOrder(3),
                      child: FocusTraversalGroup(
                        descendantsAreFocusable:
                            model.configType == LandscapeConfigType.selfHosted,
                        child: _SelfHostedForm(model),
                      ),
                    ),
                    FocusTraversalOrder(
                      order: const NumericFocusOrder(5),
                      child: FocusTraversalGroup(
                        descendantsAreFocusable:
                            model.configType == LandscapeConfigType.custom,
                        child: _CustomFileForm(model),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// A classical Back/Skip/Next button row, with the necessary callbacks.
class _NavigationButtonRow extends StatelessWidget {
  const _NavigationButtonRow({
    this.onBack,
    this.onSkip,
    this.onNext,
  });

  final void Function()? onBack;
  final void Function()? onSkip;
  final void Function()? onNext;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        OutlinedButton(
          onPressed: onBack,
          child: Text(lang.buttonBack),
        ),
        const Spacer(),
        FilledButton(
          onPressed: onSkip,
          child: Text(lang.buttonSkip),
        ),
        const SizedBox(
          width: 16.0,
        ),
        ElevatedButton(
          onPressed: onNext,
          child: Text(lang.buttonNext),
        ),
      ],
    );
  }
}

/// A selectable list tile containing a radio button, with a title and a subtitle.
class _ConfigTypeRadio extends StatelessWidget {
  const _ConfigTypeRadio({
    required this.value,
    required this.title,
    required this.subtitle,
    required this.groupValue,
    required this.onChanged,
  });
  final String title, subtitle;
  final LandscapeConfigType value, groupValue;
  final Function(LandscapeConfigType?)? onChanged;

  @override
  Widget build(BuildContext context) {
    // Adds a nice visual clue that the tile is selected.
    return YaruSelectableContainer(
      selected: groupValue == value,
      selectionColor: Theme.of(context).colorScheme.tertiaryContainer,
      child: YaruRadioListTile(
        contentPadding: EdgeInsets.zero,
        dense: true,
        title: Text(title),
        subtitle: Text(subtitle),
        value: value,
        groupValue: groupValue,
        onChanged: onChanged,
      ),
    );
  }
}

/// The subform for quick-configuring Landscape SaaS.
class _SaasForm extends StatelessWidget {
  const _SaasForm(this.model);
  final LandscapeModel model;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Column(
      children: [
        TextField(
          decoration: InputDecoration(
            label: Text(lang.landscapeAccountNameLabel),
            errorText: model.saas.accountNameError
                ? lang.landscapeAccountNameError
                : null,
          ),
          onChanged: model.setAccountName,
        ),
        const SizedBox(
          height: 8,
        ),
        TextField(
          decoration: InputDecoration(
            label: Text(lang.landscapeKeyLabel),
            hintText: '163456',
          ),
          onChanged: model.setSaasRegistrationKey,
        ),
      ],
    );
  }
}

/// The subform for quick-configuring Landscape Self-Hosted.
class _SelfHostedForm extends StatelessWidget {
  const _SelfHostedForm(this.model);
  final LandscapeModel model;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);

    return Column(
      children: [
        TextField(
          decoration: InputDecoration(
            label: Text(lang.landscapeFQDNLabel),
            errorText:
                model.selfHosted.fqdnError ? lang.landscapeFQDNError : null,
          ),
          onChanged: model.setFqdn,
        ),
        Padding(
          padding: const EdgeInsets.only(top: _kHeight),
          child: TextField(
            decoration: InputDecoration(
              label: Text(lang.landscapeKeyLabel),
              hintText: '163456',
            ),
            onChanged: model.setSelfHostedRegistrationKey,
          ),
        ),
        Padding(
          padding: const EdgeInsets.only(top: _kHeight),
          child: _FilePickerField(
            buttonLabel: lang.landscapeFilePicker,
            errorText: model.selfHosted.fileError.localize(lang),
            hint: 'C:\\landscape.pem',
            inputlabel: lang.landscapeSSLKeyLabel,
            onChanged: model.setSslKeyPath,
            allowedExtensions: const ['cer', 'crt', 'der', 'pem'],
          ),
        ),
      ],
    );
  }
}

/// The subform for passing a custom Landscape client config file.
class _CustomFileForm extends StatelessWidget {
  const _CustomFileForm(this.model);

  final LandscapeModel model;

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();
    final lang = AppLocalizations.of(context);

    return _FilePickerField(
      buttonLabel: lang.landscapeFilePicker,
      errorText: model.custom.fileError.localize(lang),
      hint: 'C:\\landscape.conf',
      inputlabel: lang.landscapeFileLabel,
      onChanged: model.setCustomConfigPath,
    );
  }
}

/// A text field with a file picker button, for selecting a file path.
class _FilePickerField extends StatefulWidget {
  const _FilePickerField({
    required this.buttonLabel,
    required this.errorText,
    required this.hint,
    required this.inputlabel,
    required this.onChanged,
    this.allowedExtensions,
  });

  final String buttonLabel, inputlabel;
  final String? errorText, hint;
  final Function(String?) onChanged;
  final List<String>? allowedExtensions;

  @override
  State<_FilePickerField> createState() => _FilePickerFieldState();
}

class _FilePickerFieldState extends State<_FilePickerField> {
  late TextEditingController txt;
  @override
  void initState() {
    super.initState();
    txt = TextEditingController();
  }

  @override
  void dispose() {
    txt.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: TextField(
            decoration: InputDecoration(
              label: Text(widget.inputlabel),
              hintText: widget.hint,
              errorText: widget.errorText,
            ),
            controller: txt,
            onChanged: widget.onChanged,
          ),
        ),
        const SizedBox(
          width: 8.0,
        ),
        FilledButton(
          onPressed: () async {
            final result = await FilePicker.platform.pickFiles(
              allowedExtensions: widget.allowedExtensions,
              type: widget.allowedExtensions == null
                  ? FileType.any
                  : FileType.custom,
            );
            if (result != null) {
              final file = File(result.files.single.path!);
              txt.text = file.path;
              widget.onChanged(file.path);
            }
          },
          child: Text(widget.buttonLabel),
        ),
      ],
    );
  }
}

/// A helper extension to localize strings matching the FileError enum.
extension FileErrorl10n on FileError {
  String? localize(AppLocalizations lang) {
    switch (this) {
      case FileError.emptyPath:
        return lang.landscapeFileEmptyPath;
      case FileError.emptyFile:
        return lang.landscapeFileEmptyContents;
      case FileError.notFound:
        return lang.landscapeFileNotFound;
      case FileError.tooLarge:
        return lang.landscapeFileTooLarge;
      case FileError.dir:
        return lang.landscapeFileIsDir;
      case FileError.none:
        return null;
    }
  }
}
