import 'package:flutter/material.dart';

/// The name of the file where the Agent's drop its service connection information.
const kAddrFileName = '.ubuntupro/.address';

/// The default border margin.
const kDefaultMargin = 32.0;

/// The path of the agent executable relative to the msix root directory.
const kAgentRelativePath = 'agent/ubuntu-pro-agent-launcher.exe';

/// The full decorated version string
const kVersion = String.fromEnvironment(
  'UP4W_FULL_VERSION',
  defaultValue: 'Dev',
);

const kConfirmColor = Color(0xFF0E8420);
