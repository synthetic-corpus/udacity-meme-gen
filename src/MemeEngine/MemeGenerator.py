""""This class creates a meme generator object.
    Its only parameter is a path of where to save 
    the generated files.
"""
from PIL import Image

class MemeGenerator:
    """Create a Meme Geneartor.
        Paramaters:
        @path = the path to save files
    """

    def __init__(self, out_path):
        self._out_path = out_path

    def make_meme(
            self,
            image_path,
            text,
            author,
            width=500) -> str:
        """Create a meme.
            @Return file path where Meme is.
        """

    @staticmethod
    def load_image(path):
        """Load an image as a file-like object."""
        image = Image.open(path)
        return image

    @staticmethod
    def add_text(image, text):
        """Add text somewhere on the image."""
        pass

    def save_image(self, image):
        """Saves image to predefined out_path."""
        pass