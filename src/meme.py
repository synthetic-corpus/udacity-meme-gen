import os
import random
import argparse
from WebEngine import ImageRequestor
from MemeEngine import MemeGenerator
from QuoteEngine import QuoteModel, Ingestor

def generate_meme(path=None, body=None, author=None):
    """ Generate a meme given an path and a quote """
    img = None
    quote = None

    if path is None:
        images = "./_data/photos/dog/"
        imgs = []
        for root, dirs, files in os.walk(images):
            imgs = [os.path.join(root, name) for name in files]

        img = random.choice(imgs)
    else:
        img = path[0]

    if body is None:
        quote_files = ['./_data/DogQuotes/DogQuotesTXT.txt',
                       './_data/DogQuotes/DogQuotesDOCX.docx',
                       './_data/DogQuotes/DogQuotesPDF.pdf',
                       './_data/DogQuotes/DogQuotesCSV.csv']
        quotes = []
        for f in quote_files:
            quotes.extend(Ingestor.parse(f))

        quote = random.choice(quotes)
    else:
        if author is None:
            raise Exception('Author Required if Body is Used')
        quote = QuoteModel(body, author)

    meme = MemeGenerator('./tmp')
    path = meme.make_meme(img, quote.body, quote.author)
    return path


if __name__ == "__main__":
    reqestor = ImageRequestor('./tmp')
    parser = argparse.ArgumentParser(
        description="Create ye a meme!"
    )

    parser.add_argument(
        'path',
        type=str,
        help='Provide a http url to the image!'
    )

    parser.add_argument(
        'body',
        type=str,
        help='Enter here the awesome quote!'
    )

    parser.add_argument(
        'author',
        type=str,
        help='Enter here the author of the awesome quote.'
    )

    args = parser.parse_args()
    args.path = reqestor.get_file(args.path)
    print(generate_meme(args.path, args.body, args.author))
