""" Test file to see if ec2 can read file and save it."""
import os
from S3engine import S3engine
from cloudlogger import log_wrapper


s3engine = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])

fonts = s3engine.list_content(folder='_fonts')

print(fonts[0])
s3engine.get_file(fonts[0][0])
print('end of test')
