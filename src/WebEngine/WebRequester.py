""" This Module handles specific web requests for content.
    This includes the initial download of content from a
    hard coded public S3 Bucket.
"""
import requests
from PIL import Image, ImageFile
from io import BytesIO
from pathlib import Path
from abc import ABC
from cloudlogger import cloud_logger, log_wrapper


class BadWebRequest(FileNotFoundError):
    """Thrown when URL are bad """
    pass


class WebRequestor(ABC):
    """ Contains Methods for Downloading Files """
    valid_file_types = []

    @log_wrapper
    def get_file(self, url) -> bytes:
        """Finds a file from the internet.
            @return provides raw bytes of images file or error
        """
        r = requests.get(url)
        if r.status_code == 200:
            file_arrayed = Path(url).name.split('.')
            print(file_arrayed)
            if file_arrayed[1] not in self.valid_file_types:
                raise TypeError(
                    f'{file_arrayed[1]} is an invalid type for this request!'
                    )
            else:
                try:
                    return r.content  # bytes
                except OSError as e:
                    print(f'Failed to save file from URL {url}')
                    raise e  # other logic may need to handle error.
        else:
            raise BadWebRequest(f'Problem with url: {url}')

    @log_wrapper
    def get_image(self, url) -> tuple[ImageFile, str]:
        try:
            b = self.get_file(url)
            image = Image.open(BytesIO(b))
            return (image, url)
        except Exception as e:
            raise e


class ImageRequestor(WebRequestor):
    """Globally ensures that only certain image formats are downloaded"""
    valid_file_types = ['jpg', 'jpeg', 'png']

    def __init__(self):
        super().__init__()


class TextRequestor(WebRequestor):
    """Globally ensures that only certain text files are downloaded"""
    valid_file_types = ['csv', 'docx', 'txt', 'pdf']

    def __init__(self):
        super().__init__()
