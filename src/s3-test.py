"""" This is a test file.
It exists soley to see if these ec2 instance can accesss s3"""

import os
import uuid
import random
from S3engine import S3engine
from PIL import Image, ImageFile

s3engine = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])

# Grab a Random Samples of the Files
some_files = s3engine.list_content()
some_files = [f[0] for f in some_files]
random.shuffle(some_files)
some_files = some_files[:5]

def make_noisy(image: ImageFile):
    print('processing noise!')
    noise_image = image.copy()
    noise_image = noise_image.convert('RGBA')
    noise_layer = Image.effect_noise(image.size, 1)
    noise_image = Image.blend(noise_image, noise_layer, 0.3)
    return noise_image


# Turns some of the files grey and then saves them
for i in some_files:
    try:
        image, name = s3engine.get_image(i)
        grey = image.convert('L')
        noise = make_noisy(image)
        ID = uuid.uuid4()
        name_grey = f'{ID}-grey.jpeg'
        name_noisy = f'{ID}-noise.jpeg'
        s3engine.put_image(image=grey, object_name=name_grey)
        print(f'Put for ${name_grey} succeeded!')
        s3engine.put_image(image=noise, object_name=name_noisy)
        print(f'Put for ${name_noisy} succeeded!')
    except Exception as e:
        print(e)
