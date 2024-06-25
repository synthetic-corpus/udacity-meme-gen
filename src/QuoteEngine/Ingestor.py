"""This File Contains a mod classes used to ingest quotes."""
import pandas
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
            raise Exception(f'Cannot import ${path} as ',
                            'one of these extentions!',
                            f'{cls.extenstions}')
        else:
            pass

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
            if len(line.text) > 0:  # Sometimes line are blank.
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
        """Each Realization will override."""
        pass


class TextIngestor(IngestorInterface):
    """Ingest the pdf format."""
    
    extenstions = ['txt']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        cls.check_extention(path)
        wise_quotes = []
        with open(path, 'r', encoding='utf-8') as text:
            word_array = text.read()
            word_array = word_array.split('\n')
            print(word_array)
            for line in word_array:
                if len(line) > 0:  # there are some blank lines
                    string = line.replace('"', '')
                    splitted_quote = string.split("-")
                    try:
                        wise_quotes.append(
                            QuoteMode(splitted_quote[0], splitted_quote[1])
                        )
                    except:
                        print(f'Likely bad input found. Line was {line}')
        return wise_quotes
