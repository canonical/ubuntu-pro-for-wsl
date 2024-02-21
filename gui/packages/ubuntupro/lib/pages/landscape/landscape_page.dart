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
      onNext: () {
        // TODO:
        // call validation
        // get file string
        // pass to agent
        model.applyManualLandscapeConfig();
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
      child: LandscapePage(),
    );
  }
}

class LandscapeInput extends StatefulWidget {
  const LandscapeInput({
    super.key,
  });

  @override
  State<LandscapeInput> createState() => _LandscapeInputState();
}

class _LandscapeInputState extends State<LandscapeInput> {
  int item = 0;

  @override
  Widget build(BuildContext context) {
    final sectionTitleStyle = Theme.of(context).primaryTextTheme.titleMedium;
    final sectionBodyStyle = Theme.of(context).primaryTextTheme.bodySmall;
    final txt = TextEditingController();

    return Column(
      mainAxisAlignment: MainAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Radio(
              value: 0,
              groupValue: item,
              onChanged: (v) {
                setState(() {
                  item = v!;
                });
              },
            ),
            const SizedBox(
              width: 16.0,
            ),
            Expanded(
              child: GestureDetector(
                onTap: () {
                  setState(() {
                    item = 0;
                  });
                },
                child: _ConfigForm(
                  sectionTitleStyle: sectionTitleStyle,
                  sectionBodyStyle: sectionBodyStyle,
                  enabled: item == 0,
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
              value: 1,
              groupValue: item,
              onChanged: (v) {
                setState(() {
                  item = v!;
                });
              },
            ),
            const SizedBox(
              width: 16.0,
            ),
            Expanded(
              child: GestureDetector(
                onTap: () {
                  setState(() {
                    item = 1;
                  });
                },
                child: _FileForm(
                  sectionTitleStyle: sectionTitleStyle,
                  sectionBodyStyle: sectionBodyStyle,
                  txt: txt,
                  enabled: item == 1,
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
            errorText: model.fqdnError ? 'Invalid URI' : null,
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
          children: [
            Expanded(
              child: TextField(
                decoration: const InputDecoration(
                  hintText: 'C:\\landscape.conf',
                ),
                enabled: enabled,
                controller: txt,
                onChanged: (value) {
                  model.path = value;
                },
                onEditingComplete: () {
                  print('finished editing');
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
