import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:ubuntupro/core/environment.dart';

void main() {
  test('with overrides', () {
    const osvalue = 'Windows_NT_X';
    final env = Environment(overrides: {'OS': osvalue});
    expect(env['OS'], osvalue);
  });
  test('without overrides', () {
    final windir = Platform.isWindows ? 'C:\\WINDOWS' : null;
    expect(Environment.instance['windir'], windir);
  });
}
