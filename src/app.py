import random
import os
from pathlib import Path
from flask import Flask, render_template, abort, request
from MemeEngine import MemeGenerator
from WebEngine import ImageRequestor
from setup import setup

app = Flask(__name__)

meme = MemeGenerator('./static')

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
