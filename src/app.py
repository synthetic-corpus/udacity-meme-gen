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

quotes = setup_text()
imgs = s3access.list_content('_sources')


@app.route('/')
def meme_rand():
    """ Generate a random meme """
    ID = uuid.uuid4()
    img_key_tup = random.choice(imgs)
    quote = random.choice(quotes)
    try:
        image_obj = s3access.get_image(img_key_tup[0])
        processed_image, image_name = meme.make_meme(image_obj, quote.body,
                                                     quote.author, uuid=ID)
        s3access.put_image(processed_image, image_name)
        return render_template('meme.html', path='bad http TODO here')
    except Exception as e:
        oops = f'{type(e).__name__} Exception: - {e}'
        cloud_logger(oops)


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
    except (BadWebRequest, OSError):
        bad_url = params['image_url']
        print(f'Could not get image from {bad_url}')
        return render_template('meme_form_error.html',
                               error_message='bad url in request'), 400
    os.remove(temp_file)

    return render_template('meme.html', path=static_location)


if __name__ == "__main__":
    app.run(host='0.0.0.0', port=80)
