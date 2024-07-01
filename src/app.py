import random
import os
import requests
from pathlib import Path
from flask import Flask, render_template, abort, request
from QuoteEngine import Ingestor
from MemeEngine import MemeGenerator
from WebEngine import ImageRequestor


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
    params = request.form
    abs_path = Path(__file__).resolve().parent
    save_path = os.path.join(abs_path, 'tmp')
    requestor = ImageRequestor(save_path)
    try:
        temp_file = requestor.get_file(params['image_url'])
        static_location = meme.make_meme(
            temp_file,
            params['body'],
            params['author']
        )
    except (UnboundLocalError):
        print(f'Could not get image from {params['image_url']}')
    os.remove(temp_file)

    return render_template('meme.html', path=static_location)


if __name__ == "__main__":
    app.run()
