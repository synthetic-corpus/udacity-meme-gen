""" This function is repsonsible for downloading
content from a public s3 bucket."""
import pandas
from WebEngine import ImageRequestor, TextRequestor

image_requests = ImageRequestor('./_data/photos')
text_request = TextRequestor('./_data/SimpleLines')


def init_from_s3():
    data_frame = pandas.read_csv('./init.csv', header=0)
    for _, row in data_frame.iterrows():
        next_import = row['url']
        print(f'importing: {next_import}')
        if row['type'] == 'img':
            image_requests.get_file(row['url'])
        else:
            text_request.get_file(row['url'])


if __name__ == "__main__":
    init_from_s3()
