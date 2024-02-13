import sys

sys.path.append('./')
from custom_conf import *
from build_requirements import *

# Configuration file for the Sphinx documentation builder.
# You should not do any modifications to this file. Put your custom
# configuration into the custom_conf.py file.
# If you need to change this file, contribute the changes upstream.
#
# For the full list of built-in configuration values, see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

############################################################
### Extensions
############################################################

extensions = [
    'sphinx_design',
    'sphinx_copybutton',
    'sphinxcontrib.jquery',
]

# Only add redirects extension if any redirects are specified.
if AreRedirectsDefined():
    extensions.append('sphinx_reredirects')

# Only add myst extensions if any configuration is present.
if IsMyStParserUsed():
    extensions.append('myst_parser')

    # Additional MyST syntax
    myst_enable_extensions = [
        'substitution',
        'deflist',
        'linkify'
    ]
    myst_enable_extensions.extend(custom_myst_extensions)

# Only add Open Graph extension if any configuration is present.
if IsOpenGraphConfigured():
    extensions.append('sphinxext.opengraph')
    
extensions.extend(custom_extensions)
extensions = DeduplicateExtensions(extensions)

### Configuration for extensions

# Used for related links
if not 'discourse_prefix' in html_context and 'discourse' in html_context:
    html_context['discourse_prefix'] = html_context['discourse'] + '/t/'

# Default image for OGP (to prevent font errors, see
# https://github.com/canonical/sphinx-docs-starter-pack/pull/54 )
if not 'ogp_image' in locals():
    ogp_image = 'https://assets.ubuntu.com/v1/253da317-image-document-ubuntudocs.svg'

############################################################
### General configuration
############################################################

exclude_patterns = [
    '_build',
    'Thumbs.db',
    '.DS_Store',
    '.sphinx',
]
exclude_patterns.extend(custom_excludes)

rst_epilog = '''
.. include:: /reuse/links.txt
'''
if 'custom_rst_epilog' in locals():
    rst_epilog = custom_rst_epilog

source_suffix = {
    '.rst': 'restructuredtext',
    '.md': 'markdown',
}

if not 'conf_py_path' in html_context and 'github_folder' in html_context:
    html_context['conf_py_path'] = html_context['github_folder']

# For ignoring specific links
linkcheck_anchors_ignore_for_url = [
    r'https://github\.com/.*'
]
linkcheck_anchors_ignore_for_url.extend(custom_linkcheck_anchors_ignore_for_url)

# Tags cannot be added directly in custom_conf.py, so add them here
for tag in custom_tags:
    tags.add(tag)

############################################################
### Styling
############################################################

# Find the current builder
builder = 'dirhtml'
if '-b' in sys.argv:
    builder = sys.argv[sys.argv.index('-b')+1]

# Setting templates_path for epub makes the build fail
if builder == 'dirhtml' or builder == 'html':
    templates_path = ['.sphinx/_templates']

# Theme configuration
html_theme = 'furo'
html_last_updated_fmt = ''
html_permalinks_icon = '¶'

if html_title == '':
    html_theme_options = {
        'sidebar_hide_name': True
        }

############################################################
### Additional files
############################################################

html_static_path = ['.sphinx/_static']

html_css_files = [
    'custom.css',
    'header.css',
    'github_issue_links.css',
    'furo_colors.css'
]
html_css_files.extend(custom_html_css_files)

html_js_files = ['header-nav.js']
if 'github_issues' in html_context and html_context['github_issues'] and not disable_feedback_button:
    html_js_files.append('github_issue_links.js')
html_js_files.extend(custom_html_js_files)
