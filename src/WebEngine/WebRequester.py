""" This Module handles specific web requests for content.
    This includes the initial download of content from a
    hard coded public S3 Bucket.
"""
import pickle
import hashlib
import requests
from pathlib import Path
from abc import ABC


class WebRequestor(ABC):
    """ Contains Methods for Downloading Files """
    valid_file_types = []

    def __init__(self, save_folder):
        """Save Folder is the location where files
            Will be saved.
        """
        self.save_folder = save_folder

    @classmethod
    def name_by_hash(cls, data):
        """Returns the name of the image file as a Hash.
            Useful for uniqueness.
        """
        image_bytes = pickle.dumps(data)
        hasher = hashlib.sha256()
        hasher.update(image_bytes)
        hash_name = hasher.hexdigest()
        return hash_name

    def get_file(self, url) -> str:
        """Finds a file from the internet.
            @return provides a local path name of file.
        """
        r = requests.get(url)
        if r.status_code == 200:
            file_arrayed = Path(url).name.split('.')
            if file_arrayed[1] not in self.valid_file_types:
                raise TypeError(
                    f'{file_arrayed[1]} is an invalid type for this request!'
                    )
            else:
                file_arrayed[0] = self.name_by_hash(r.content)
                file_name = ''.join(file_arrayed)
                try:
                    with open(f'{self.save_folder}/{file_name}') as file:
                        file.write(r.content)
                except OSError as e:
                    print('Failed to save file:')
                    print(f'{self.save_folder}/{file_name}')
                    raise e  # other logic may need to handle error.


class ImageRequestor(WebRequestor):
    """Globally ensures that only certain image formats are downloaded"""
    valid_file_types = ['jpg', 'jpeg', 'png']

    def __init__(self, save_folder):
        super().__init__(save_folder)


class TextRequestor(WebRequestor):
    """Globally ensures that only certain image formats are downloaded"""
    valid_file_types = ['csv', 'docx', 'txt', 'pdf']

    def __init__(self, save_folder):
        super().__init__(save_folder)
