from flask import Flask, render_template, request

app = Flask(__name__)


@app.route('/')
def meme_rand():
    """ Generate a random meme """
    try:
        return render_template('meme.html')
    except Exception as e:
        oops = f'{type(e).__name__} Exception: - {e}'
        return oops, 500



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
    try:
        print(params)

        return render_template('meme.html', path=url_path)
    except Exception as e:
        """ This comment exists to change a hash at deployment """
        bad_url = params['image_url']
        print(f'Could not get image from {bad_url}')
        oops = f'{type(e).__name__} Exception: - {e}'
        print(oops)
        return render_template('meme_form_error.html',
                               error_message='bad url in request'), 400


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)

