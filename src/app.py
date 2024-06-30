import random
import os
import requests
from flask import Flask, render_template, abort, request
from QuoteEngine import Ingestor
from MemeEngine import MemeGenerator

# @TODO Import your Ingestor and MemeEngine classes

app = Flask(__name__)

meme = MemeGenerator('./static')

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

    quote_files = get_files('./_data','txt','docx','csv')
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


quotes, imgs = setup()


@app.route('/')
def meme_rand():
    """ Generate a random meme """

    img = random.choice(imgs)
    quote = random.choice(quotes)
    path = meme.make_meme(img, quote.body, quote.author)
    return render_template('meme.html', path=path)


@app.route('/create', methods=['GET'])
def meme_form():
    """ User input for meme information """
    return render_template('meme_form.html')


@app.route('/create', methods=['POST'])
def meme_post():
    """ Create a user defined meme """

    # @TODO:
    # 1. Use requests to save the image from the image_url
    #    form param to a temp local file.
    # 2. Use the meme object to generate a meme using this temp
    #    file and the body and author form paramaters.
    # 3. Remove the temporary saved image.

    path = None

    return render_template('meme.html', path=path)


if __name__ == "__main__":
    app.run()
