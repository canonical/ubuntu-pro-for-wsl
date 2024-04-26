import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wizard_router/wizard_router.dart';

import '/core/agent_api_client.dart';
import '/pages/widgets/page_widgets.dart';
import 'landscape_model.dart';

class LandscapePage extends StatelessWidget {
  const LandscapePage({
    super.key,
    required this.onApplyConfig,
    this.onSkip,
    this.onBack,
  });

  final void Function() onApplyConfig;
  final void Function()? onSkip;
  final void Function()? onBack;

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();
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

    return ColumnLandingPage(
      svgAsset: 'assets/Landscape-tag.svg',
      title: 'Landscape',
      onNext: !model.hasError
          ? () async {
              if (await model.applyConfig() && context.mounted) {
                onApplyConfig();
              }
            }
          : null,
      onBack: onBack ?? () => Wizard.of(context).back(),
      onSkip: onSkip ?? () => Wizard.of(context).next(),
      leftChildren: [
        MarkdownBody(
          data: lang.landscapeHeading('[Landscape](${model.landscapeURI})'),
          onTapLink: (_, href, __) => launchUrl(model.landscapeURI),
          styleSheet: linkStyle,
        ),
      ],
      children: const [
        LandscapeInput(),
      ],
    );
  }

  static Widget create(BuildContext context, {bool isLate = false}) {
    final client = getService<AgentApiClient>();
    LandscapePage landscapePage;
    if (isLate) {
      landscapePage = LandscapePage(
        onApplyConfig: () => Wizard.of(context).back(),
        onSkip: () => Wizard.of(context).back(),
      );
    } else {
      landscapePage = LandscapePage(
        onApplyConfig: () => Wizard.of(context).next(),
      );
    }

    return ChangeNotifierProvider<LandscapeModel>(
      create: (context) => LandscapeModel(client),
      child: landscapePage,
    );
  }
}

class LandscapeInput extends StatelessWidget {
  const LandscapeInput({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final sectionTitleStyle = theme.textTheme.titleMedium;
    final sectionBodyStyle = theme.textTheme.bodySmall;
    final model = context.watch<LandscapeModel>();

    return Column(
      mainAxisAlignment: MainAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Radio(
              value: LandscapeConfigType.manual,
              groupValue: model.selected,
              onChanged: (v) {
                model.selected = v!;
              },
            ),
            const SizedBox(
              width: 16.0,
            ),
            Expanded(
              child: GestureDetector(
                onTap: () {
                  model.selected = LandscapeConfigType.manual;
                },
                child: _ConfigForm(
                  sectionTitleStyle: sectionTitleStyle,
                  sectionBodyStyle: sectionBodyStyle,
                  enabled: model.selected == LandscapeConfigType.manual,
                ),
              ),
            ),
          ],
        ),
        const SizedBox(
          height: 32.0,
        ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Radio(
              value: LandscapeConfigType.file,
              groupValue: model.selected,
              onChanged: (v) {
                model.selected = v!;
              },
            ),
            const SizedBox(
              width: 16.0,
            ),
            Expanded(
              child: GestureDetector(
                onTap: () {
                  model.selected = LandscapeConfigType.file;
                },
                child: _FileForm(
                  sectionTitleStyle: sectionTitleStyle,
                  sectionBodyStyle: sectionBodyStyle,
                  enabled: model.selected == LandscapeConfigType.file,
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _ConfigForm extends StatefulWidget {
  const _ConfigForm({
    required this.sectionTitleStyle,
    required this.sectionBodyStyle,
    required this.enabled,
  });

  final TextStyle? sectionTitleStyle;
  final TextStyle? sectionBodyStyle;
  final bool enabled;

  @override
  State<_ConfigForm> createState() => _ConfigFormState();
}

class _ConfigFormState extends State<_ConfigForm> {
  late TextEditingController accountNameController;

  @override
  void initState() {
    super.initState();
    accountNameController = TextEditingController();
    final model = context.read<LandscapeModel>();
    model.addListener(() {
      accountNameController.text = model.accountName;
    });
  }

  @override
  void dispose() {
    accountNameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();
    final lang = AppLocalizations.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          lang.landscapeQuickSetup,
          style: widget.sectionTitleStyle,
        ),
        Text(
          lang.landscapeQuickSetupHint,
          style: widget.sectionBodyStyle,
        ),
        const SizedBox(
          height: 16.0,
        ),
        TextField(
          enabled: widget.enabled,
          decoration: InputDecoration(
            label: Text(lang.landscapeFQDNLabel),
            hintText: LandscapeModel.landscapeSaas,
            errorText: model.fqdnError && widget.enabled
                ? lang.landscapeFQDNError
                : null,
          ),
          onChanged: (value) {
            model.fqdn = value;
          },
        ),
        const SizedBox(
          height: 8.0,
        ),
        TextField(
          enabled: widget.enabled && model.canEnterAccountName,
          controller: accountNameController,
          decoration: InputDecoration(
            label: Text(lang.landscapeAccountNameLabel),
            hintText:
                model.canEnterAccountName ? null : LandscapeModel.standalone,
            errorText: model.accountNameError && widget.enabled
                ? lang.landscapeAccountNameError
                : null,
          ),
          onChanged: (value) {
            model.accountName = value;
          },
        ),
        const SizedBox(
          height: 8.0,
        ),
        TextField(
          enabled: widget.enabled,
          decoration: InputDecoration(
            label: Text(lang.landscapeKeyLabel),
            hintText: '123456',
          ),
          onChanged: (value) {
            model.key = value;
          },
        ),
      ],
    );
  }
}

class _FileForm extends StatefulWidget {
  const _FileForm({
    required this.sectionTitleStyle,
    required this.sectionBodyStyle,
    required this.enabled,
  });

  final TextStyle? sectionTitleStyle;
  final TextStyle? sectionBodyStyle;
  final bool enabled;

  @override
  State<_FileForm> createState() => _FileFormState();
}

class _FileFormState extends State<_FileForm> {
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
    final model = context.watch<LandscapeModel>();
    final lang = AppLocalizations.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          lang.landscapeCustomSetup,
          style: widget.sectionTitleStyle,
        ),
        Text(
          lang.landscapeCustomSetupHint,
          style: widget.sectionBodyStyle,
        ),
        const SizedBox(
          height: 16.0,
        ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: TextField(
                decoration: InputDecoration(
                  label: Text(lang.landscapeFileLabel),
                  hintText: 'C:\\landscape.conf',
                  errorText: model.fileError != FileError.none && widget.enabled
                      ? model.fileError.localize(lang)
                      : null,
                ),
                enabled: widget.enabled,
                controller: txt,
                onChanged: (value) {
                  model.path = value;
                },
              ),
            ),
            const SizedBox(
              width: 8.0,
            ),
            FilledButton(
              onPressed: widget.enabled
                  ? () async {
                      final result = await FilePicker.platform.pickFiles();
                      if (result != null) {
                        final file = File(result.files.single.path!);
                        txt.text = file.path;
                        model.path = file.path;
                      }
                    }
                  : null,
              child: Text(lang.landscapeFilePicker),
            ),
          ],
        ),
      ],
    );
  }
}

extension FileErrorl10n on FileError {
  String localize(AppLocalizations lang) {
    switch (this) {
      case FileError.empty:
        return lang.landscapeFileEmpty;
      case FileError.notFound:
        return lang.landscapeFileNotFound;
      case FileError.tooLarge:
        return lang.landscapeFileTooLarge;
      default:
        throw UnimplementedError(toString());
    }
  }
}
