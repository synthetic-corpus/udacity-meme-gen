"""This class is for listing, saving, and accessing
Files from a particular s3 bucket.
It is expected it should handle ImageFile objects without
saving them locally to a drive as a file unless specifically
requested"""
import boto3
from PIL import Image
from PIL.ImageFile import ImageFile
from io import BytesIO
from cloudlogger import log_wrapper, cloud_logger


class S3engine:
    def __init__(self, s3_Bucket, aws_region):
        """Gets access to a specific bucket"""
        self.my_s3 = boto3.resource('s3', region_name=aws_region)
        self.my_bucket = self.my_s3.Bucket(s3_Bucket)

    @log_wrapper
    def list_content(self, folder='_sources') -> list[tuple[str, str]]:
        """gets all content from specific s3 folder"""
        """Returns both a key and file name per file"""
        try:
            objects = list(self.my_bucket.objects.filter(Prefix=folder))
            sources = [(o.key, o.key.split('/')[1]) for o in objects]
            message = f'Returning a list for {folder}. Sample {sources[:2]}'
            cloud_logger.info(message)
            return sources
        except Exception as e:
            cloud_logger.error(f'{type(e).__name__} - {e}')

    @log_wrapper
    def get_image(self, object_key) -> tuple[ImageFile, str]:
        """Returns an ImageFile and original name of file."""
        try:
            s3_object = self.my_bucket.Object(object_key)
            response = s3_object.get()
            file_stream = response['Body']
            this_image = Image.open(file_stream)
            """ return the original name of the file too """
            return (this_image, object_key.split("/")[1])
        except Exception as e:
            cloud_logger.error(f'{type(e).__name__} - {e}')

    @log_wrapper
    def put_image(self, image: ImageFile,
                  object_name: str, folder='_images') -> None:
        """Takes an image file, its name, and saves it to s3"""
        try:
            object_key = f'{folder}/{object_name}'
            new_s3_object = self.my_bucket.Object(object_key)
            file_stream = BytesIO()
            image.save(file_stream, format='jpeg')
            new_s3_object.put(Body=file_stream.getvalue())
            # TODO at this point, the file should return the public URL
            # of the image.
        except Exception as e:
            cloud_logger.error(f'{type(e).__name__} - {e}')
