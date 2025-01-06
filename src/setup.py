""" This is a set up script
    To be run by either app.py
    or meme.py as needed.
"""
import os
from QuoteEngine import Ingestor


def verify_result(func, path: str, *extensions) -> list[str]:
    """Wraps a get file function.
        If nothing is there, will run init_from_s3
    """
    all_files = func(path, *extensions)
    if len(all_files) == 0:
        print(f'No files found for {extensions}.')
        all_files = func(path, *extensions)
        return all_files
    return all_files


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


def setup_text():
    """ Sets up quotes only """
    data_path = './_data'

    quote_files = get_files(data_path, 'txt', 'docx', 'csv', 'pdf')
    print(quote_files)
    all_quotes = []
    for file_path in quote_files:
        try:
            more_quotes = Ingestor.parse(file_path)
        except TypeError as e:
            print(
                (f'Invalid file type ${file_path}'),
                (e)
            )
        else:
            all_quotes.extend(more_quotes)

    return all_quotes
