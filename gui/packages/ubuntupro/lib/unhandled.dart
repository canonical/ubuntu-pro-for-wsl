import 'package:flutter/foundation.dart';
import 'package:ubuntu_logger/ubuntu_logger.dart';

final _log = Logger('unhandled');

void setUpUnhandledErrors() {
  FlutterError.onError = (details) {
    FlutterError.presentError(details);
    _log.error(details);
  };

  PlatformDispatcher.instance.onError = (error, stack) {
    _log.error(error);
    _log.error(stack);
    return true;
  };
}
