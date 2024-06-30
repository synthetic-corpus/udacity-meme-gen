""""This class creates a meme generator object.
    Its only parameter is a path of where to save 
    the generated files.
"""
from random import randint
import pickle
import hashlib
from PIL import Image, ImageDraw, ImageFont

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

    @classmethod
    def rightsize_text(cls, text: str, font: ImageFont, draw: ImageDraw, max_width):
        """Adds line breaks at max width."""
        words = text.split()
        lines = []
        current_line = []
        current_width = 0
        for word in words:
            w_width = draw.textlength(word, font=font)
            if current_width + w_width <= max_width:
                current_line.append(word)
                current_width = w_width + draw.textlength(' ', font=font) + current_width
            else:
                new_line = ' '.join(current_line)
                new_line = new_line + " \n"
                lines.append(new_line)
                current_line = [word]
                current_width = w_width + draw.textlength(' ', font=font)
        if len(current_line) > 0:
            new_line = ' '.join(current_line)
            new_line = new_line + " \n"
            lines.append(new_line)
        multi_line_text = "".join(lines)
        return multi_line_text



    @classmethod
    def name_by_hash(cls, image: Image):
        """Returns the name of the image file as a Hash.
            Useful for uniqueness.
        """
        image_bytes = pickle.dumps(image)
        hasher = hashlib.sha256()
        hasher.update(image_bytes)
        hash_name = hasher.hexdigest()
        return hash_name

    @staticmethod
    def load_image(path) -> Image:
        """Load an image as a file-like object."""
        image = Image.open(path)
        return image

    def add_text(self, image: Image, text: str, font_name='Arial.ttf', font_size=20) -> None:
        """Add text somewhere on the image."""
        if font_size < 12 or font_size > 40:
            print(
                ('Font Size must be between 12 and 40.'),
                ('Reverting to default size of 20.')
                )
            font_size = 20
        writer = ImageDraw.Draw(image)
        try:
            font = ImageFont.truetype(font_name, font_size)
        except OSError as e:
            print(
                (f'Unable to locate font {font_name}'),
                ('Using default font Arial.ttf'),
                (e)
                )
            font = ImageFont.truetype('Arial.ttf', font_size)

        random_horizontal = randint(10, image.width // 3)  # Not too far over to right.
        random_vertical = randint(10, int(image.height * .7))  # Never too close to the bottom.
        text = MemeGenerator.rightsize_text(text, font, writer, image.width - 30 - random_horizontal)
        writer.text((random_horizontal, random_vertical), text, fill='white', font=font)

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