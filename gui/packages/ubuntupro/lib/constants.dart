/// The address where the Agent gRPC service is hosted.
const kDefaultHost = '127.0.0.1';

/// This app's name. Needed to find the Agent's addr file.
const kAppName = 'Ubuntu Pro';

/// The name of the file where the Agent's drop its service connection information.
const kAddrFileName = 'addr';

/// The default border margin.
const kDefaultMargin = 32.0;

/// The consistent duration fur quick animations throughout the app.
const kQuickAnimationDuration = Duration(milliseconds: 250);

/// The path of the agent executable relative to the msix root directory.
const kAgentRelativePath = 'agent/ubuntu-pro-agent.exe';
