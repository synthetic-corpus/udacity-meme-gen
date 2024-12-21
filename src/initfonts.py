"""
    This install fonts stored from a s3 bucket.
    Run only if the installation of fonts is actually needed
"""
from pathlib import Path
import os
import subprocess
from S3engine import S3engine

font_path = '/usr/share/fonts/truetype'
s3engine = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])

fonts = s3engine.list_content('_fonts')

for font_keys in fonts:
    font, font_name = s3engine.get_file(font_keys[0])
    save_here = os.path.join(font_path,font_name)
    with open(save_here, 'wb') as f:
        f.write(font.read())
    print(f'Saved Font: {font_name}')

refresh_command = ['fc-cache','-fv']
result = subprocess.run(refresh_command, 
               capture_output=True, 
               text=True)

print(result.stderr)
print(result.stdout)
print("all Done")