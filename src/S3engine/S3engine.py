"""This class is for listing, saving, and accessing
Files from a particular s3 bucket.
It is expected it should handle ImageFile objects without
saving them locally to a drive as a file unless specifically
requested"""
import boto3
from PIL import Image
from PIL.ImageFile import ImageFile
import os
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
            """ Ignore the first element """
            """ First element is '(folder,"")' which is not useful"""
            return sources[1:]
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
            # Return the constructed url of the images
            domain = os.environ['CDN']
            file_url = f'{domain}/_images/{object_name}'
            return file_url
        except Exception as e:
            cloud_logger.error(f'{type(e).__name__} - {e}')

    @log_wrapper
    def get_file(self, file_key: str):
        """ Intended to get a generic file """
        s3_object = self.my_bucket.Object(file_key)
        response = s3_object.get()
        file_stream = response['Body']
        """ return the original name of the file too """
        return (file_stream, file_key.split("/")[1])

    @log_wrapper
    def load_fonts(self, folder='/usr/share/fonts') -> str:
        """ This call loads fonts from s3 to an ec2 instance """
        fonts = self.list_content('_fonts')
        output_array = []
        for font_tuple in fonts:
            font, font_name = self.get_file(font_tuple[0])
            save_here = os.path.join(folder, font_name)
            try:
                with open(save_here, 'wb') as f:
                    f.write(font.read())
                output_array.append(font_name)
            except Exception as e:
                cloud_logger.error(f'could not load font {font_name} - {e}')
        message = f'Succesfully loaded fonts: {output_array}'
        return message  # this is jut for easy logging

    @log_wrapper
    def load_quotes(self):
        """ Loads quotes into a local folder on ec2 """
        quotes = self.list_content('_text')
        current_folder = os.getcwd()
        relative_path = '/../_data/miniquotes/'
        sources = []
        for quote_tuple in quotes:
            quote_data, quote_name = self.get_file(quote_tuple[0])
            save_here = os.path.join(current_folder, relative_path, quote_name)
            try:
                with open(save_here, 'wb') as f:
                    f.write(quote_data.read())
                    sources.append(quote_name)
            except Exception as e:
                cloud_logger.error(f'could not load text! {quote_name}')
        message = f'Loaded text data: {sources}'
        return message  # easy logging again.
