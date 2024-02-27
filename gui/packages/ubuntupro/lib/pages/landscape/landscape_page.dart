import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_gen/gen_l10n/app_localizations.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';

import '/core/agent_api_client.dart';
import '/pages/widgets/page_widgets.dart';
import 'landscape_model.dart';

class LandscapePage extends StatelessWidget {
  const LandscapePage({super.key});

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();
    final lang = AppLocalizations.of(context);
    final linkStyle = MarkdownStyleSheet.fromTheme(
      Theme.of(context).copyWith(
        textTheme: DarkStyledLandingPage.textTheme.copyWith(
          bodyMedium: DarkStyledLandingPage.textTheme.bodyMedium?.copyWith(
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
      onNext: () async {
        if (!model.validConfig()) {
          return false;
        }
        final success = await model.applyConfig();
        return success;
      },
      leftChildren: [
        MarkdownBody(
          data: lang
              .landscapeHeading('[Landscape](https://ubuntu.com/landscape)'),
          onTapLink: (_, href, __) => model.launchLandscapeWebPage(),
          styleSheet: linkStyle,
        ),
      ],
      children: const [
        LandscapeInput(),
      ],
    );
  }

  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    return ChangeNotifierProvider<LandscapeModel>(
      create: (context) => LandscapeModel(client),
      child: const LandscapePage(),
    );
  }
}

class LandscapeInput extends StatefulWidget {
  const LandscapeInput({super.key});

  @override
  State<LandscapeInput> createState() => _LandscapeInputState();
}

class _LandscapeInputState extends State<LandscapeInput> {
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
    final sectionTitleStyle = Theme.of(context).primaryTextTheme.titleMedium;
    final sectionBodyStyle = Theme.of(context).primaryTextTheme.bodySmall;
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
                  txt: txt,
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _ConfigForm extends StatelessWidget {
  const _ConfigForm({
    required this.sectionTitleStyle,
    required this.sectionBodyStyle,
    required this.enabled,
  });

  final TextStyle? sectionTitleStyle;
  final TextStyle? sectionBodyStyle;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Quick Setup (Recommended)',
          style: sectionTitleStyle,
        ),
        Text(
          'Provide your Landscape enrollment information',
          style: sectionBodyStyle,
        ),
        const SizedBox(
          height: 16.0,
        ),
        TextField(
          enabled: enabled,
          decoration: InputDecoration(
            label: const Text('Landscape FQDN'),
            hintText: 'landscape.canonical.com',
            errorText: model.fqdnError && enabled ? 'Invalid URI' : null,
          ),
          onChanged: (value) {
            model.fqdn = value;
          },
        ),
        const SizedBox(
          height: 8.0,
        ),
        TextField(
          enabled: enabled,
          decoration: const InputDecoration(
            label: Text('Landscape Account Name'),
            hintText: 'standalone',
          ),
        ),
        const SizedBox(
          height: 8.0,
        ),
        TextField(
          enabled: enabled,
          decoration: const InputDecoration(
            label: Text('Registration Key'),
            hintText: '123456',
          ),
        ),
      ],
    );
  }
}

class _FileForm extends StatelessWidget {
  const _FileForm({
    required this.sectionTitleStyle,
    required this.sectionBodyStyle,
    required this.txt,
    required this.enabled,
  });

  final TextStyle? sectionTitleStyle;
  final TextStyle? sectionBodyStyle;
  final TextEditingController txt;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final model = context.watch<LandscapeModel>();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Custom Configuration',
          style: sectionTitleStyle,
        ),
        Text('Load a custom client configuration file',
            style: sectionBodyStyle),
        const SizedBox(
          height: 16.0,
        ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: TextField(
                decoration: InputDecoration(
                  hintText: 'C:\\landscape.conf',
                  errorText: model.fileError && enabled ? 'Invalid file' : null,
                ),
                enabled: enabled,
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
              onPressed: enabled
                  ? () async {
                      final result = await FilePicker.platform.pickFiles();
                      if (result != null) {
                        final file = File(result.files.single.path!);
                        txt.text = file.path;
                        model.path = file.path;
                      }
                    }
                  : null,
              child: const Text('Select file...'),
            ),
          ],
        )
      ],
    );
  }
}
