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

    @staticmethod
    def scale_image(image: Image, new_width: int) -> Image:
        """Scale an image by width maintaint aspect ratio"""
        height = image.height
        width = image.width
        percentage = new_width / width
        new_height = int(height * percentage)

        resized = image.resize((new_width, new_height))
        return resized

    def save_image(self, image):
        """Saves image to predefined out_path."""
        pass