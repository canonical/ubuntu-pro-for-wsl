const uap5ns = 'http://schemas.microsoft.com/appx/manifest/uap/windows10/5';
const agentPath = 'agent\\ubuntu-pro-agent.exe';
const manifestFileName = 'AppxManifest.xml';
const extensionsToAdd = '''<Extensions>
        <uap5:Extension Category="windows.startupTask" Executable="$agentPath" EntryPoint="Windows.FullTrustApplication">
          <uap5:StartupTask TaskId="P4W_agent" Enabled="true" DisplayName="Ubuntu Pro For Windows background agent" />
        </uap5:Extension>
        <desktop:Extension Category="windows.fullTrustProcess" Executable="$agentPath">
          <desktop:FullTrustProcess>
            <desktop:ParameterGroup GroupId="agent" Parameters="" />
          </desktop:FullTrustProcess>
        </desktop:Extension>
      </Extensions>
''';
