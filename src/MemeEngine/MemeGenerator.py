""""This class creates a meme generator object.
    Its only parameter is a path of where to save 
    the generated files.
"""

import pickle
import hashlib
from random import randint
from PIL import Image, ImageDraw, ImageFont

class MemeGenerator:
    """Create a Meme Geneartor.
        Paramaters:
        @path = the path to save files
    """

    def __init__(self, out_folder):
        self._out_path = out_folder

    def make_meme(
            self,
            image_path,
            text,
            author,
            font='Arial.ttf',
            width=500) -> str:
        """Create a meme.
            @Return file path where Meme is.
        """
        try:
            next_image = self.load_image(image_path)
        except OSError as e:  # Creates a 'file not found' image
            print(e)
            blank_square = Image.new('RGB', (500, 500), color='white')
            writer = ImageDraw.Draw(blank_square)
            font = ImageFont.truetype('Arial.ttf', 40)
            message = f'File: \n {image_path} \n not found!'
            length = writer.textlength(text, font)
            horizontal = (500 - length) // 2
            vertical = 250
            writer.text((horizontal, vertical),
                        message,
                        fill='black',
                        font=font)
            file_path = f'{self._out_path}/not-found.jpg'
            blank_square.save(file_path)
            next_image = self.load_image(file_path)

        next_image = self.scale_image(next_image, width)
        text = f'"{text}" - {author}'
        self.add_text(next_image, text, font_name='Arial.ttf', font_size=30)
        image_name = f'{self.name_by_hash(next_image)}.jpg'
        file_path = f'{self._out_path}/{image_name}'
        next_image.save(file_path)
        return file_path

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