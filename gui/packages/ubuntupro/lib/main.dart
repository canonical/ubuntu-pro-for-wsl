import 'package:flutter/material.dart';
import 'package:windows_single_instance/windows_single_instance.dart';
import 'package:yaru_widgets/yaru_widgets.dart';
import 'app.dart';

Future<void> main() async {
  await YaruWindowTitleBar.ensureInitialized();
  await WindowsSingleInstance.ensureSingleInstance(
    [],
    'UP4W_SINGLE_INSTANCE_GUI',
  );
  runApp(const Pro4WSLApp());
}
