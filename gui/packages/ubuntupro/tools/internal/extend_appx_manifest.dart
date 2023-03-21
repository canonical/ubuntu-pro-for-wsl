import 'dart:io';

import 'package:logging/logging.dart';
import 'package:xml/xml.dart';

import 'manifest_constants.dart';

Future<bool> extendAppxManifest(String buildDir) async {
  final log = Logger('Extend Appx Manifest');
  final manifestPath = '$buildDir/$manifestFileName';

  final manifest = File(manifestPath);
  if (!manifest.existsSync()) {
    log.severe(
      "Couldn't find the $manifestFileName file. Did you run `flutter pub run msix:build` ?",
    );
    return false;
  }
  try {
    final contents = await manifest.readAsString();
    if (contents.isEmpty) {
      log.severe(
        '$manifestFileName cannot be empty. Consider running `flutter clean && flutter pub run msix:build`',
      );
      return false;
    }

    final doc = XmlDocument.parse(contents);

    doc.rootElement.addNamespace('uap5', uap5ns);

    final apps = doc.findAllElements('Application');
    if (apps.length != 1) {
      log.severe('$manifestFileName should contain exactly one <Application>.');
      return false;
    }

    final app = apps.first;
    if (app.findAllElements('Extensions').isNotEmpty) {
      log.warning('Expected manifest to contain no app extensions');
      return false;
    }

    app.applyAppExtension(extensionsToAdd);

    await manifest.writeAsString(doc.toXmlString(pretty: true));
    log.info('AppxManifest updated successfully.');
    return true;
  } on XmlException catch (err) {
    log.severe('Invalid XML: ${err.message}');
    return false;
  } on FileSystemException catch (err) {
    log.severe(
      'Something went wrong when trying to read the file. ${err.message}',
    );
    return false;
  }
}

extension _XmlPro on XmlElement {
  void addNamespace(String key, String value) {
    assert(!hasParent, 'Only the root element can add namespaces');
    attributes.add(
      XmlAttribute(XmlName(key, 'xmlns'), value),
    );
    attributes.sort((a, b) => a.localName.compareTo(b.localName));
  }

  void applyAppExtension(String xmlContent) {
    assert(
      localName == 'Application',
      'Extensions must be applied only to the <Application> element.',
    );
    final extensions = XmlDocument.parse(xmlContent);
    children.add(extensions.getElement('Extensions')!.copy());
  }
}
