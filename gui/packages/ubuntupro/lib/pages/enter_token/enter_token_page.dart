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

  @override
  Widget build(BuildContext context) {
    final model = context.watch<EnterProTokenModel>();
    return Scaffold(
      appBar: YaruWindowTitleBar(
        title: Text(title),
      ),
      body: Padding(
        padding: const EdgeInsets.all(kDefaultMarging),
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: <Widget>[
              TextField(
                decoration: InputDecoration(
                  labelText: 'Paste here your Ubuntu Pro token',
                  errorText: model.errorOrNull?.localize(),
                ),
                onChanged: model.update,
              ),
              const SizedBox(height: kDefaultMarging),
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
