import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:ubuntu_service/ubuntu_service.dart';
import 'package:yaru_widgets/yaru_widgets.dart';

import '../../constants.dart';
import '../../core/agent_api_client.dart';
import 'enter_token_model.dart';

class EnterProTokenPage extends StatelessWidget {
  const EnterProTokenPage({super.key, required this.title});

  final String title;
  static Widget create(BuildContext context) {
    final client = getService<AgentApiClient>();
    return ChangeNotifierProvider(
      create: (_) => EnterProTokenModel(client),
      child: const EnterProTokenPage(title: 'Ubuntu Pro For Windows'),
    );
  }

  double? textFieldWidth(BuildContext context) {
    final fontSize = Theme.of(context).textTheme.bodySmall?.fontSize;
    if (fontSize == null) {
      return null;
    }
    final textScale = MediaQuery.of(context).textScaleFactor;
    return maxTokenWidth(
      fontSize: fontSize,
      textScale: textScale,
    );
  }

  @override
  Widget build(BuildContext context) {
    final model = context.watch<EnterProTokenModel>();
    return Scaffold(
      appBar: YaruWindowTitleBar(
        title: Text(title),
      ),
      body: Padding(
        padding: const EdgeInsets.all(kDefaultMargin),
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              SizedBox(
                width: textFieldWidth(context),
                child: TextField(
                  decoration: InputDecoration(
                    labelText: 'Paste here your Ubuntu Pro token',
                    errorText: model.errorOrNull?.localize(),
                    counterText: '',
                  ),
                  onChanged: model.update,
                ),
              ),
              const SizedBox(height: kDefaultMargin),
              ElevatedButton(
                onPressed: model.hasError
                    ? null
                    : () {
                        model.apply();
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(
                            content: Text('Applying token ${model.token}'),
                          ),
                        );
                      },
                child: const Text('Apply Pro Token'),
              )
            ],
          ),
        ),
      ),
    );
  }
}
