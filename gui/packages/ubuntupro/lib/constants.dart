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

/// The environment variable users should set to enable integration with Landscape.
const kLandscapeAllowedEnvVar = 'UP4W_ALLOW_LANDSCAPE_INTEGRATION';
