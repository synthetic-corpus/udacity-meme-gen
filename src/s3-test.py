"""" This is a test file.
It exists soley to see if these ec2 instance can accesss s3"""

import boto3
from PIL import Image, ImageDraw
from io import BytesIO
import random
import os


class S3tester:
    """Creates a connection to an S3 Bucket"""
    """Handles Read/Write to said bucket"""
    def __init__(self, bucket_name, region_name):
        """Takes in the name of the bucket and its region"""
        self.my_s3 = boto3.resource('s3', region_name=region_name)
        self.my_bucket = self.my_s3.Bucket(bucket_name)

    @staticmethod
    def rename_file(file_name, suffix='_processed'):
        """appends a suffix to a file name"""
        """e.g. 'myfile.jpeg -> myfile_processed.jpeg"""
        pivot = file_name.rfind('.')
        array = [file_name[:pivot], suffix, '.jpeg']
        return ''.join(array)

    def get_image(self, object_name):
        """ Gets file from the s3 and returns it as an Image """
        object_key = f'_sources/{object_name}'
        s3_object = self.my_bucket.Object(object_key)
        response = s3_object.get()
        file_stream = response['Body']
        this_image = Image.open(file_stream)
        """ return the original name of the file too """
        return (this_image, object_name)

    def write_image(self, image: Image, object_name):
        new_name = s3tester.rename_file(file_name=object_name)
        object_key = f'_images/{new_name}'
        s3_object = self.my_bucket.Object(object_key)
        file_stream = BytesIO()
        image.save(file_stream, format='jpeg')
        s3_object.put(Body=file_stream.getvalue())

    def get_sample(self, size=3):
        """ gets some random source files """
        keys = self.my_s3.objects.filter()
        sources = [key for key in keys if '_source' in key]
        random.shuffle(sources)
        return sources[:size]


bucket, region = os.environ['SOURCE_S3'], os.environ['SOURCE_REGION']
s3tester = S3tester(bucket, region)
random_images = s3tester.get_sample()

for i in random_images:
    image, name = s3tester.get_image(i)
    grey = image.convert('L')
    s3tester.write_image(grey, name)
