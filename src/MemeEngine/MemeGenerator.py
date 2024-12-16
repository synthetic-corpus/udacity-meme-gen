""""This class creates a meme generator object.
    Its only parameter is a path of where to save
    the generated files.

    Meme Generator now takes a ImageFile as an inpute
    and outputs the same.
"""

from random import randint
from PIL import Image, ImageDraw, ImageFont
from PIL.ImageFile import ImageFile


class MemeGenerator:
    """Create a Meme Geneartor.
        Paramaters:
        @path = the folder of the s3 bucket.
    """

    def __init__(self, out_folder):
        self._out_path = out_folder

    def make_meme(
            self,
            source_file,
            text,
            author,
            uuid,
            font='Arial.ttf',
            width=500) -> tuple[ImageFile, str]:
        """Create a meme.
            @source_file: a loaded ImageFile, from s3.
            @Return an ImageFile that is the result of
            processing
        """

        source_file = self.scale_image(source_file, width)
        text = f'"{text}" - {author}'
        self.add_text(image=source_file, text=text,
                      font_name='Arial.ttf', font_size=30)

        image_name = f'{uuid}-text.jpeg'
        return (source_file, image_name)

    @classmethod
    def rightsize_text(cls, text: str,
                       font: ImageFont,
                       draw: ImageDraw,
                       max_width):
        """Adds line breaks at max width."""
        words = text.split()
        lines = []
        current_line = []
        current_width = 0
        for word in words:
            w_width = draw.textlength(word, font=font)
            if current_width + w_width <= max_width:
                current_line.append(word)
                current_width = w_width + \
                    draw.textlength(' ', font=font) + \
                    current_width
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

    @staticmethod
    def load_image(path) -> Image:
        """Load an image as a file-like object."""
        image = Image.open(path)
        return image

    def add_text(self, image: Image, text: str,
                 font_name='Arial.ttf', font_size=20) -> None:
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

        # Not too close to the right.
        r_horizontal = randint(10, image.width // 3)
        # Never too close to the bottom.
        r_vertical = randint(10, int(image.height * .7))
        max_w = image.width - 30 - r_horizontal
        text = MemeGenerator.rightsize_text(text, font,
                                            writer, max_w)
        writer.text((r_horizontal, r_vertical),
                    text, fill='white', font=font)

    @staticmethod
    def scale_image(image: Image, new_width: int) -> Image:
        """Scale an image by width maintaint aspect ratio"""
        height = image.height
        width = image.width
        percentage = new_width / width
        new_height = int(height * percentage)

        resized = image.resize((new_width, new_height))
        return resized
