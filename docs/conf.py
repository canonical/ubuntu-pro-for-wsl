import datetime
import os
import textwrap

# Configuration for the Sphinx documentation builder.
# All configuration specific to your project should be done in this file.
#
# If you're new to Sphinx and don't want any advanced or custom features,
# just go through the items marked 'TODO'.
#
# A complete list of built-in Sphinx configuration values:
# https://www.sphinx-doc.org/en/master/usage/configuration.html
#
# The Sphinx Stack uses the Canonical Sphinx theme to keep all documentation consistent
# and on brand:
# https://github.com/canonical/canonical-sphinx

#######################
# Project information #
#######################

# Project name
#
# Update with the official name of your project or product

project = "Ubuntu on WSL"
author = "Canonical Ltd."

# The year in the copyright statement
copyright = f"{datetime.date.today().year}"

# Sidebar documentation title
# To disable the title, set it to an empty string.
html_title = project + " documentation"

# Documentation website URL
#
# Update with the official URL of your docs or leave empty if unsure.
#
# NOTE: The Open Graph Protocol (OGP) enhances page display in a social graph
#       and is used by social media platforms; see https://ogp.me/

version = f"{os.environ.get('READTHEDOCS_VERSION', 'local')}"

ogp_site_url = f"https://ubuntu.com/wsl/docs/{version}"


# Preview name of the documentation website
# To use a different name for the project in previews, update the next line.
ogp_site_name = project

# Preview image URL
# To customise the preview image, update the next line.
ogp_image = "https://assets.ubuntu.com/v1/cc828679-docs_illustration.svg"

# Product favicon; shown in bookmarks, browser tabs, etc.

# To customise the favicon, uncomment and update as needed.

# html_favicon = '.sphinx/_static/favicon.png'


# Dictionary of values to pass into the Sphinx context for all pages:
# https://www.sphinx-doc.org/en/master/usage/configuration.html#confval-html_context
html_context = {
    # Product page URL; can be different from product docs URL
    #
    # Change to your product website URL,
    #       dropping the 'https://' prefix, e.g. 'ubuntu.com/lxd'.
    #
    # If there's no such website,
    #       remove the {{ product_page }} link from the page header template
    #       (usually .sphinx/_templates/header.html; also, see README.rst).
    "product_page": "ubuntu.com/wsl",
    # Product tag image; the orange part of your logo, shown in the page header
    # To add a tag image, uncomment and update as needed.
    # 'product_tag': '_static/tag.png',
    # Your Discourse instance URL
    # Change to your Discourse instance URL or leave empty.
    #
    # NOTE: If set, adding ':discourse: 123' to an .rst file
    #       will add a link to Discourse topic 123 at the bottom of the page.
    "discourse": "https://discourse.ubuntu.com/c/wsl/27",
    # Your Mattermost channel URL
    # Change to your Mattermost channel URL or leave empty.
    "mattermost": "",
    # Your Matrix channel URL
    # Change to your Matrix channel URL or leave empty.
    "matrix": "https://matrix.to/#/#ubuntu-wsl:ubuntu.com",
    # Your documentation GitHub repository URL
    #
    # Change to your documentation GitHub repository URL or leave empty.
    #
    # NOTE: If set, links for viewing the documentation source files
    #       and creating GitHub issues are added at the bottom of each page.
    "github_url": "https://github.com/canonical/ubuntu-pro-for-wsl",
    # Docs branch in the repo; used in links for viewing the source files
    "repo_default_branch": "main",
    # Docs location in the repo; used in links for viewing the source files
    "repo_folder": "/docs/",
    # To enable or disable the Previous / Next buttons at the bottom of pages
    # Valid options: none, prev, next, both
    "sequential_nav": "both",
    # To enable listing contributors on individual pages, set to True
    "display_contributors": False, # deprecated
    # Required for feedback button
    "github_issues": "enabled",
    "author": author,
    "license": {
        # Specify your project's license.
        # For the name, we recommend using the standard shorthand identifier from
        # https://spdx.org/licenses
        "name": "CC-BY-SA", # NOTE: using as the docs license
        # Link directly to your project's license statement.
        "url": "https://creativecommons.org/licenses/by-sa/4.0/deed.en",
    },
}

# To enable the edit button on pages, uncomment and change the link to a
# public repository on GitHub or Launchpad. Any of the following link domains
# are accepted:
# - https://github.com/example-org/example"
# - https://launchpad.net/example
# - https://git.launchpad.net/example
#
# html_theme_options = {
#     "source_edit_link": "https://github.com/canonical/ubuntu-pro-for-wsl",
# }
# NOTE: not enabling edit button because it doesn't handle versioned docs

# Project slug; see https://meta.discourse.org/t/what-is-category-slug/87897
#
# If your documentation is hosted on https://docs.ubuntu.com/,
#       uncomment and update as needed.

slug = "wsl/docs"

#######################
# Sitemap configuration: https://sphinx-sitemap.readthedocs.io/
#######################

# Use RTD canonical URL to ensure duplicate pages have a specific canonical URL

html_baseurl = f"https://ubuntu.com/wsl/docs/{version}/"

# sphinx-sitemap uses html_baseurl to generate the full URL for each page:

sitemap_url_scheme = '{link}'

# Include `lastmod` dates in the sitemap:

sitemap_show_lastmod = True

# Exclude generated pages from the sitemap:

sitemap_excludes = [
    '404/',
    'genindex/',
    'search/',
]

# Add more pages to sitemap_excludes if needed. Wildcards are supported.
#       For example, to exclude module pages generated by autodoc, add '_modules/*'.

################################
# Template and asset locations #
################################

html_static_path = ["_static"]
templates_path = ["_templates"]


#############
# Redirects #
#############

# Add redirects to the 'redirects.txt' file
# https://sphinxext-rediraffe.readthedocs.io/en/latest/

# To set up redirects in the Read the Docs project dashboard:
# https://docs.readthedocs.io/en/stable/guides/redirects.html

# To set up redirects when a page has moved or been renamed, use the redirects.txt file
rediraffe_redirects = "redirects.txt"

# NOTE: for automated checking of changed/moved URLS --- note enabled (yet)
# rediraffe_branch = "main"

# Strips '/index.html' from destination URLs when building with 'dirhtml'
rediraffe_dir_only = True


############################
# sphinx-llm configuration #
############################

# This description is included in llms.txt to provide some initial context for your
# product docs.
# Add a description in the form "This is the documentation for <product name>,
# <first sentence of home page>".
llms_txt_description = textwrap.dedent(
    """\
    This is the documentation for Ubuntu on WSL, the default Linux distribution for WSL, and the Pro for WSL application for managing and securing instances of Ubuntu on WSL.
    """
)

# The base URL for references built by sphinx-markdown-builder.
if os.environ.get("READTHEDOCS"):
    markdown_http_base = html_baseurl


###########################
# Link checker exceptions #
###########################

# A regex list of URLs that are ignored by 'make linkcheck'
linkcheck_ignore = [
    "http://127.0.0.1:8000",
    # Linkcheck does not have access to the repo
    "https://github.com/canonical/ubuntu-pro-for-wsl/*",
    # This page redirects to SSO login:
    "https://ubuntu.com/pro/dashboard",
    # Only users logged in to MS Store with their account registered for beta can access this link
    "https://apps.microsoft.com/detail/9PD1WZNBDXKZ",
    # Linkcheck struggles with hashes in URLs
    "https://matrix.to/#/#documentation:ubuntu.com",
    # Server is rejecting automated requests
    "https://www.freedesktop.org/software/*",
]


# A regex list of URLs where anchors are ignored by 'make linkcheck'
linkcheck_anchors_ignore_for_url = [
    r"https://github\.com/.*",
    r"https://learn.microsoft\.com/.*",
]

# give linkcheck multiple tries on failure
# linkcheck_timeout = 30
linkcheck_retries = 3

########################
# Configuration extras #
########################

# Custom MyST syntax extensions; see
# https://myst-parser.readthedocs.io/en/latest/syntax/optional.html
# NOTE: By default, the following MyST extensions are enabled:
#   - substitution
#   - deflist
#   - linkify
myst_enable_extensions = set({"colon_fence"})

# Custom Sphinx extensions; see
# https://www.sphinx-doc.org/en/master/usage/extensions/index.html
extensions = [
    "canonical_sphinx",
    "notfound.extension",
    "sphinx_design",
    "sphinx_rerediraffe",
    "sphinx_tabs.tabs",
    "sphinxcontrib.jquery",
    "sphinxext.opengraph",
    "sphinx_config_options",
    "sphinx_contributor_listing",
    "sphinx_filtered_toctree",
    "sphinx_llm.txt",
    "sphinx_related_links",
    "sphinx_roles",
    "sphinx_terminal",
    "sphinx_ubuntu_images",
    "sphinx_youtube_links",
    "sphinxcontrib.cairosvgconverter",
    "sphinx_last_updated_by_git",
    "sphinx.ext.intersphinx",
    "sphinx_sitemap",
    "sphinxcontrib.mermaid",
]

# Excludes files or directories from processing
exclude_patterns = [
    ".venv*",
    "internal",
]

# Adds custom CSS files, located under 'html_static_path' or remotely
html_css_files = [
        "css/pro_block.css",
        "css/dropdown.css",
        "css/mermaid-custom.css"
        "https://assets.ubuntu.com/v1/d86746ef-cookie_banner.css",
        ]


# Adds custom JavaScript files, located under 'html_static_path' or remotely
html_js_files = [
        "https://assets.ubuntu.com/v1/287a5e8f-bundle.js",
]

# Appends extra markup to the end of every document written in reST
# rst_epilog = """
# """

# Feedback button at the top; enabled by default
# Disable the button if your project is unsuitable for public feedback.
# disable_feedback_button = True

# Your manpage URL
# To enable manpage links, uncomment and replace {codename} with required
#       release, preferably an LTS release (e.g. noble). Do *not* substitute
#       {section} or {page}; these will be replaced by sphinx at build time
#
# NOTE: If set, adding ':manpage:' to an .rst file
#       adds a link to the corresponding man section at the bottom of the page.
# manpages_url = 'https://manpages.ubuntu.com/manpages/{codename}/en/' + \
#     'man{section}/{page}.{section}.html'

# Specifies a reST snippet to be prepended to each .rst file
# This defines a :center: role that centers table cell content.
# This defines a :h2: role that styles content for use with PDF generation.
rst_prolog = """
.. role:: center
   :class: align-center
.. role:: h2
    :class: hclass2
.. role:: woke-ignore
    :class: woke-ignore
.. role:: vale-ignore
    :class: vale-ignore
"""

# Workaround for https://github.com/canonical/canonical-sphinx/issues/34

if "discourse_prefix" not in html_context and "discourse" in html_context:
    html_context["discourse_prefix"] = html_context["discourse"] + "/t/"

# Configuration for Intersphinx projects
#
# intersphinx_mapping = {
#     "snap": ("https://snapcraft.io/docs/", None),
# }

# Define a selector that only adds copy buttons to code blocks without the class `no-copy`
copybutton_selector = "div:not(.no-copy) > div.highlight > pre"

# TODO: deprecate in favor of terminal extension
# Define prompts to be excluded from copying when a copy button is used
copybutton_prompt_text = r"^.*?[\$>]\s+"
copybutton_prompt_is_regexp = True

# Enables GitHub-compatible syntax for diagrams
myst_fence_as_directive = ["mermaid"]
