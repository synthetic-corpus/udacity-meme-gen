"""This File Contains a mod classes used to ingest quotes."""
import re
import pandas
import subprocess
from docx import Document
from abc import ABC, abstractmethod
from QuoteModel import QuoteMode


class IngestorInterface(ABC):
    """Is the Abstract class fo ingesters."""

    extenstions = []

    @classmethod
    def check_extention(cls, path: str) -> None:
        """Check the file extension for a match."""
        extension = path.split('.')[-1]
        if (extension not in cls.extenstions):
            raise TypeError(f'Cannot import ${path} as ',
                            'one of these extentions!',
                            f'{cls.extenstions}',
                            TypeError)
        else:
            pass

    @classmethod
    def validate_line(cls, line) -> bool:
        """Validate a single line."""
        regex_pattern = r'"([^"]*) - [a-zA-Z]'
        if re.Match(regex_pattern, line) is None:
            # To do throw a custom exception
            print(f'{line} is invlaid will not be added!')
            return False
        return True

    @classmethod
    def validate_csv(cls, dataframe: pandas.DataFrame):
        """Ensure the CSV file has the correct headers."""
        expected_headers = ['body', 'author']
        if dataframe.colums.tolist() != expected_headers:
            # To do throw a custom exception
            return False
        return True

    @classmethod
    @abstractmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass


class CSVIngestor(IngestorInterface):
    """Ingest the CSV format."""

    extenstions = ['csv']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Ingests quotes from file to a list."""
        cls.check_extention(path)
        dataframe = pandas.read_csv(path, header=0)
        wise_quotes = []  # to be returned
        for _, row in dataframe.iterrows():
            wise_quotes.append(
                QuoteMode(row['body'], row['author'])
            )
        return wise_quotes


class DocxIngestor(IngestorInterface):
    """Ingest the docx format."""

    extenstions = ['docx']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Iterate over each docx line and create Quotes."""
        cls.check_extention(path)
        doc = Document(path)
        wise_quotes = []
        for line in doc.paragraphs:
            if cls.validate_input(line.text):  # Sometimes line are blank.
                string = line.text.replace('"', '')
                array = string.split("-")
                wise_quotes.append(
                    QuoteMode(array[0], array[1])
                )
        return wise_quotes


class PDFIngestor(IngestorInterface):
    """Ingest the pdf format."""

    extenstions = ['pdf']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Ingests a pdf. Converts to text"""
        cls.check_extention(path)
        outfile_path = '../_data/tmp/pdf-as-textf.txt'
        subprocess.call([
            'pdftotext', '-enc', 'UTF-8', '-simple',
            path, outfile_path
            ])
        # TXT expected to be compatible with
        # this other ingestor.
        return TextIngestor.ingest(outfile_path)


class TextIngestor(IngestorInterface):
    """Ingest the txt format."""

    extenstions = ['txt']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Open text file and process."""
        cls.check_extention(path)
        wise_quotes = []
        with open(path, 'r', encoding='utf-8') as text:
            word_array = text.read()
            word_array = word_array.split('\n')
            print(word_array)
            for line in word_array:
                if cls.validate_line(line):  # there are some blank lines
                    string = line.replace('"', '')
                    splitted_quote = string.split("-")
                    try:
                        wise_quotes.append(
                            QuoteMode(splitted_quote[0], splitted_quote[1])
                        )
                    except SyntaxError:
                        print(
                            ('Likely bad input found.'),
                            (f'Line was {line}. {SyntaxError}')
                            )
        return wise_quotes


class IngestAny(IngestorInterface):
    """Ingests any of the four possible file types."""

    extenstions = ['docx', 'csv', 'pdf', 'txt']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Ingest any vaild filetype."""
        try:
            cls.check_extention(path)
        except TypeError as exc:
            raise exc
        extension = path.split('.')[-1]

        match extension:
            case 'docx':
                return DocxIngestor.ingest(path)
            case 'csv':
                return CSVIngestor.ingest(path)
            case 'pdf':
                return PDFIngestor.ingest(path)
            case 'txt':
                return TextIngestor.ingest(path)
