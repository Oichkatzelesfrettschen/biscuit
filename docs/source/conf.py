import os
import sys

project = 'Biscuit'

extensions = ['breathe']

breathe_projects = {
    'Biscuit': os.path.join('..', 'build', 'xml')
}

breathe_default_project = 'Biscuit'
html_theme = 'sphinx_rtd_theme'

