""" This is a set up script
    To be run by either app.py
    or meme.py as needed.
"""
import os
from QuoteEngine import Ingestor

def get_files(path: str, *extensions: str) -> list[str]:
    """Get all files from path that
        match specified exentions."""
    all_files = []
    for (path, _, files) in os.walk(path):
        for file in files:
            ext = file.split('.')[-1]
            if ext in extensions:
                file_path = os.path.join(path, file)
                all_files.append(file_path)
    return all_files



def setup():
    """ Load all resources """

    quote_files = get_files('./_data','txt','docx','csv','pdf')
    print(quote_files)
    all_quotes = []
    for file_path in quote_files:
        try:
            more_quotes =  Ingestor.parse(file_path)
        except TypeError as e:
            print(
                (f'Invalid file type ${file_path}'),
                (e)
            )
        else:
            all_quotes.extend(more_quotes)

    image_path = './_data'
    imgs = get_files(image_path,'jpg','jpeg','png')

    return all_quotes, imgs