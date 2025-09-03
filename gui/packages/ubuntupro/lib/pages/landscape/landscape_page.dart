import 'dart:async';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:grpc/grpc.dart' show StatusCode;
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wizard_router/wizard_router.dart';
import 'package:yaru/yaru.dart';

import '/constants.dart';
import '/core/agent_api_client.dart';
import '/l10n/app_localizations.dart';
import '/pages/widgets/delayed_text_field.dart';
import '/pages/widgets/navigation_row.dart';
import '/pages/widgets/page_widgets.dart';
import 'landscape_model.dart';

/// Defines the overall structure of the Landscape configuration page and seggregates
/// the portions of the page that must rebuild at the relevant state changes.
class LandscapePage extends StatelessWidget {
  const LandscapePage({
    super.key,
    required this.onApplyConfig,
    required this.onBack,
  });

  /// Callable invoked when this page successfully applies the configuration.
  final void Function() onApplyConfig;

  /// Callable invoked when the user navigates back.
  final void Function() onBack;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final theme = Theme.of(context);
    final model = context.watch<LandscapeModel>();
    final linkStyle = MarkdownStyleSheet.fromTheme(
      Theme.of(context).copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w100,
          ),
        ),
      ),
    );

    return ColumnPage(
      svgAsset: 'assets/Landscape-tag.svg',
      title: kLandscapeTitle,
      rightIsCentered: false,
      left: [
        MarkdownBody(
          data: lang.landscapeHeading(
            '[${lang.learnMore}](${LandscapeModel.landscapeURI})',
          ),
          onTapLink: (_, href, __) => launchUrl(LandscapeModel.landscapeURI),
          styleSheet: linkStyle,
        ),
      ],
      right: [const SizedBox(height: 24), LandscapeConfigForm(model)],
      navigationRow: NavigationRow(
        onBack: onBack,
        onNext: model.isComplete && !model.isWaiting
            ? () => _tryApplyConfig(context)
            : null,
        // Overlaying the spinner on top of the hidden text preserves the button size.
        next: Stack(
          alignment: Alignment.center,
          children: [
            Opacity(
              opacity: model.isWaiting ? 0 : 1.0,
              child: Text(lang.landscapeRegister),
            ),
            if (model.isWaiting)
              const SizedBox(
                height: 16,
                width: 16,
                child: CircularProgressIndicator(),
              )
          ],
        ),
      ),
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

    final lang = AppLocalizations.of(context);
    final actions = [
      FilledButton(
        onPressed: Navigator.of(context).pop,
        child: Text(lang.landscapeEditYourConfig),
      ),
      ElevatedButton(
        style: ElevatedButton.styleFrom(backgroundColor: YaruColors.red),
        onPressed: onApplyConfig,
        child: Text(lang.landscapeProceedAnyway),
      ),
    ];

    var title = '';
    var content = '';
    switch (err.code) {
      case StatusCode.ok:
        onApplyConfig();
        return;
      case StatusCode.permissionDenied:
        title = lang.landscapeWSLUnavailable;
        content = lang.landscapeWSLUnavailableContent;
        break;
      case StatusCode.unavailable:
        title = lang.landscapeUnreachable;
        content = lang.landscapeUnreachableContent;
        break;
      case StatusCode.invalidArgument:
        title = lang.landscapeInvalidConfig;
        content = lang.landscapeInvalidConfigContent;
        break;
      case StatusCode.alreadyExists:
        title = lang.landscapeUnchangedConfig;
        content = lang.landscapeUnchangedConfigContent;
        break;
      case StatusCode.unknown:
        title = lang.landscapeUnknownError;
        content = lang.landscapeUnknownErrorContent;
        break;
      default:
        title = lang.landscapeUnknownError;
        content = lang.landscapeUnknownErrorContent;
        break;
    }

    await showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) {
        return LandscapeErrorDialog(
          title: title,
          content: content,
          details: err.message ?? '',
          actions: actions,
        );
      },
    );
  }

  /// Creates a new Landscape page with its associated model connected to the Wizard
  static Widget create(BuildContext context, {bool isLate = false}) {
    final client = getService<AgentApiClient>();
    LandscapePage landscapePage;
    if (isLate) {
      landscapePage = LandscapePage(
        onApplyConfig: Wizard.of(context).back,
        onBack: Wizard.of(context).back,
      );
    } else {
      landscapePage = LandscapePage(
        onApplyConfig: Wizard.of(context).next,
        onBack: Wizard.of(context).back,
      );
    }

    return ChangeNotifierProvider<LandscapeModel>(
      create: (context) => LandscapeModel(client),
      child: landscapePage,
    );
  }
}

class LandscapeErrorDialog extends StatelessWidget {
  const LandscapeErrorDialog({
    super.key,
    required this.title,
    required this.content,
    required this.details,
    required this.actions,
  });

  final String title;
  final String content;
  final String details;
  final List<ButtonStyleButton> actions;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    return AlertDialog(
      title: Text(title),
      content: SizedBox(
        width: 640,
        child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(content),
              const SizedBox(height: 8),
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 200),
                child: YaruExpansionPanel(
                  shrinkWrap: true,
                  // Padding below is the default, I'm just setting it now to prevent misaligned contents
                  // in case default changes in the future.
                  headerPadding: const EdgeInsets.only(left: 20),
                  headers: [Text(lang.landscapeDetails)],
                  children: [
                    Padding(
                      padding: const EdgeInsets.fromLTRB(20.0, 0, 20.0, 8.0),
                      child: Row(
                        children: [Expanded(child: SelectableText(details))],
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              Text(
                lang.landscapeProceedAnywayHint,
              ),
              const SizedBox(height: 4),
              Text(
                lang.landscapeChangeLaterHint,
              ),
            ]),
      ),
      actions: actions,
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

    return Column(
      children: [
        YaruRadioListTile(
          value: LandscapeConfigType.manual,
          groupValue: model.configType,
          contentPadding: EdgeInsets.zero,
          onChanged: model.setConfigType,
          title: Text(lang.landscapeSetupManual),
          subtitle: Text(lang.landscapeSetupManualHint),
        ),
        const SizedBox(height: 8),
        _ManualForm(model),
        const SizedBox(height: 24),
        YaruRadioListTile(
          value: LandscapeConfigType.custom,
          groupValue: model.configType,
          contentPadding: EdgeInsets.zero,
          onChanged: model.setConfigType,
          title: Text(lang.landscapeSetupCustom),
          subtitle: Text(lang.landscapeSetupCustomHint),
        ),
        const SizedBox(height: 8),
        _CustomFileForm(model),
      ],
    );
  }
}

/// The subform for quick-configuring Landscape Manual.
class _ManualForm extends StatelessWidget {
  const _ManualForm(this.model);
  final LandscapeModel model;

  @override
  Widget build(BuildContext context) {
    final lang = AppLocalizations.of(context);
    final enabled = model.configType == LandscapeConfigType.manual;

    return Column(
      children: [
        DelayedTextField(
          label: Text(lang.landscapeFQDNLabel),
          errorText: enabled ? model.manual.fqdnError.localize(lang) : null,
          onChanged: model.setFqdn,
          enabled: enabled,
        ),
        const SizedBox(height: 8),
        DelayedTextField(
          label: Text(lang.landscapeKeyLabel),
          hintText: '163456',
          onChanged: model.setManualRegistrationKey,
          enabled: enabled,
        ),
        const SizedBox(height: 8),
        _FilePickerField(
          buttonLabel: lang.landscapeFilePicker,
          errorText: enabled ? model.manual.fileError.localize(lang) : null,
          hint: 'C:\\landscape.pem',
          inputlabel: lang.landscapeSSLKeyLabel,
          onChanged: model.setSslKeyPath,
          allowedExtensions: validCertExtensions,
          enabled: enabled,
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
    final enabled = model.configType == LandscapeConfigType.custom;

    return _FilePickerField(
      buttonLabel: lang.landscapeFilePicker,
      errorText: enabled ? model.custom.fileError.localize(lang) : null,
      hint: 'C:\\landscape.conf',
      inputlabel: lang.landscapeFileLabel,
      onChanged: model.setCustomConfigPath,
      enabled: enabled,
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
    this.enabled = true,
    this.allowedExtensions,
  });

  final String buttonLabel, inputlabel;
  final String? errorText, hint;
  final Function(String?) onChanged;
  final List<String>? allowedExtensions;
  final bool enabled;

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
          child: DelayedTextField(
            label: Text(widget.inputlabel),
            hintText: widget.hint,
            errorText: widget.errorText,
            controller: txt,
            onChanged: widget.onChanged,
            enabled: widget.enabled,
          ),
        ),
        const SizedBox(width: 8.0),
        FilledButton(
          onPressed: widget.enabled
              ? () async {
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
                }
              : null,
          child: Text(widget.buttonLabel),
        ),
      ],
    );
  }
}

/// A helper extension to localize strings matching the FileError enum.
extension FileErrorL10n on FileError {
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
      case FileError.invalidFormat:
        return lang.landscapeFileInvalidFormat;
      case FileError.none:
        return null;
    }
  }
}

/// Helper to localize FQDN error strings.
extension FQDNErrorL10n on FqdnError {
  String? localize(AppLocalizations lang) {
    switch (this) {
      case FqdnError.invalid:
        return lang.landscapeFQDNError;
      case FqdnError.none:
        return null;
      case FqdnError.saas:
        return lang.landscapeFQDNSaaSError;
    }
  }
}
