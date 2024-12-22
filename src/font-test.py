""" Test file to see if ec2 can read file and save it."""
import os
from S3engine import S3engine
from cloudlogger import log_wrapper
import subprocess
import uuid
import random
from MemeEngine import MemeGenerator


s3engine = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])
m = MemeGenerator('_sources')

fonts = s3engine.list_content(folder='_fonts')
img_key = s3engine.list_content('_sources')
fonts = s3engine.list_content('_fonts')
font_path = '/usr/share/fonts'
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

font_cmd = ['fc-list','--format=%{file}\n']
result = subprocess.run(font_cmd,
                       capture_output=True,
                       text=True)

fonts = result.stdout.split('\n')
fonts = [x.split('/')[-1] for x in fonts if 'google' not in x]
fonts = [x for x in fonts if x != '']
images = s3engine.list_content('_sources')

print("My Possible Fonts...")
print(fonts)

""" Just do this ten times"""
ID = uuid.uuid4()
img_key = random.choice(images)[0]
font = random.choice(fonts)
quote = f'This thing written in... {font} !'

inputs = f'Font = {font}, id = {ID}, text= {quote}'
try:
    image_obj, _ = s3engine.get_image(img_key)
    drawn, name = m.make_meme(
        source_file=image_obj,
        text=quote,
        author='anon',
        uuid=ID,
        font=font
    )
    s3engine.put_image(drawn, name)
    print('wrote with:')
    print(inputs)
except Exception as e:
    print(inputs)
    print(e)
