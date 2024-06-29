""" This Class to be an extension of the
Pillow Image class.

All functions (text writing, resizing etc)
to be accomplished in OOP way.
"""
from PIL.Image import Image

class MemeImage(Image):
    def __init__(self):
        Image.__init__(self)