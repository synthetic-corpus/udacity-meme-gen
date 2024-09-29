"""
    This install fonts.
    Run only if the installation of fonts is actually needed
"""
from pathlib import Path
import os
import shutil

abs_path = Path(__file__).resolve().parent

font_source = f'{abs_path}/_fonts'
font_dest = '/usr/share/fonts'
for font in os.listdir(font_source):
    source_file = os.path.join(font_source, font)
    shutil.copy(source_file, font_dest)
    print(f'Copied {source_file} to host font folder')
