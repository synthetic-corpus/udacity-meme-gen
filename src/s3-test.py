"""" This is a test file.
It exists soley to see if these ec2 instance can accesss s3"""

import os
import uuid
import random
from S3engine import S3engine
from PIL import Image, ImageFile, ImageFilter, ImageEnhance
from cloudlogger import log_wrapper
from DatabaseAccess import DatabaseAccess

s3engine = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])
db = DatabaseAccess()

# Grab a Random Samples of the Files
some_files = s3engine.list_content()
some_files = [f[0] for f in some_files]
random.shuffle(some_files)
some_files = some_files[:5]


queue = []
# Turns some of the files grey and then saves them
for i in some_files:
    image, name = s3engine.get_image(i)
    ID = uuid.uuid4()

    try:
        print(f'Making Blurry: {name}')
        blurry = image.filter(ImageFilter.BoxBlur(radius=12))
        blurry_name = f'{ID}-blurry.jpeg'
        queue.append((blurry, blurry_name))
        grey = image.convert('L')
    except Exception as e:
        print(e)

    try:
        print(f'Making Grey: {name}')
        grey = image.convert("1")
        grey_name = f'{ID}-grey.jpeg'
        queue.append((grey, grey_name))
    except Exception as e:
        print(e)

    try:
        print(f'Making Loud: {name}')
        enhancer = ImageEnhance.Color(image)
        loud = enhancer.enhance(2.0)
        loud_name = f'{ID}-loud.jpeg'
        queue.append((loud, loud_name))
    except Exception as e:
        print(e)

    # Simple DB test
    outputs = [t[1] for t in queue]
    db.record_processing(id=ID, source=name, ouputs=outputs)

for t in queue:
    """Save them to Images """
    s3engine.put_image(image=t[0],object_name=t[1])