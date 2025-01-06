import random
import uuid
import os
from PIL.ImageFile import ImageFile
from pathlib import Path
from flask import Flask, render_template, make_response, request
from MemeEngine import MemeGenerator
from WebEngine import ImageRequestor, BadWebRequest
from S3engine import S3engine
from cloudlogger import cloud_logger
from setup import setup_text

app = Flask(__name__)

meme = MemeGenerator('_sources')
s3access = S3engine(os.environ['S3_BUCKET'], os.environ['SOURCE_REGION'])
print('now loading quotes..')
path = os.path.join(os.path.dirname(__file__), '_data/miniquotes')
s3access.load_quotes(path)
quotes = setup_text()
print('now prepping source images...')
imgs = s3access.list_content('_sources')
print('now loading fonts...')
s3access.load_fonts()


@app.route('/')
def meme_rand():
    """ Generate a random meme """
    ID = uuid.uuid4()
    img_key_tup = random.choice(imgs)
    quote = random.choice(quotes)
    my_font = MemeGenerator.random_font()
    try:
        image_obj, _ = s3access.get_image(img_key_tup[0])
        processed_image, image_name = meme.make_meme(
            source_file=image_obj,
            text=quote.body,
            author=quote.author,
            font=my_font,
            uuid=ID)
        url_path = s3access.put_image(processed_image, image_name)
        if url_path.find('https://') == -1:
            """ Sanitizing input, basically..."""
            url_path = f'https://{url_path}'
        return render_template('meme.html', path=url_path)
    except Exception as e:
        oops = f'{type(e).__name__} Exception: - {e}'
        cloud_logger.info(oops)


@app.route('/health', methods=['GET'])
def health_check():
    """ Health Check for AWS services """
    return "OK", 200


@app.route('/create', methods=['GET'])
def meme_form():
    """ User input for meme information """
    return render_template('meme_form.html')


@app.route('/create', methods=['POST'])
def meme_post():
    """ Create a user defined meme """
    params = request.form
    requestor = ImageRequestor('_images')
    try:
        web_image = requestor.get_image(params['image_url'])
        ID = uuid.uuid4()
        my_font = MemeGenerator.random_font()
        processed_image, image_name = meme.make_meme(
            source_file=web_image,
            text=params['body'],
            author=params['author'],
            uuid=ID,
            font=my_font
        )
        url_path = s3access.put_image(processed_image, image_name)
        if url_path.find('https://') == -1:
            """ Sanitizing input, basically..."""
            url_path = f'https://{url_path}'
        return render_template('meme.html', path=url_path)
    except (BadWebRequest, OSError):
        bad_url = params['image_url']
        print(f'Could not get image from {bad_url}')
        return render_template('meme_form_error.html',
                               error_message='bad url in request'), 400


if __name__ == "__main__":
    app.run(host='0.0.0.0', port=80)
