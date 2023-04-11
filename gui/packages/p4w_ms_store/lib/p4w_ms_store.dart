import 'p4w_ms_store_platform_interface.dart';

class P4wMsStore {
  /// Launches the full-trust process associated with the current app as
  /// manifested in the application package, passing the command line arguments
  /// specified by [args], if any.
  ///
  /// For Windows that association is made via the AppxManifest declaring the
  /// `windows.fullTrustProcess` extension. There is no equivalent on Linux,
  /// thus the Linux implementation must be just a mock process.
  ///
  /// See:
  /// <https://learn.microsoft.com/en-us/uwp/schemas/appxpackage/uapmanifestschema/element-desktop-extension>
  /// <https://learn.microsoft.com/en-us/uwp/api/windows.applicationmodel.fulltrustprocesslauncher>
  Future<void> launchFullTrustProcess({List<String>? args}) {
    return P4wMsStorePlatform.instance.launchFullTrustProcess(args);
  }
}
