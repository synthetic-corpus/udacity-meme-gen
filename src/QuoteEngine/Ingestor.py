"""This File Contains a mod classes used to ingest quotes."""
import re
import pandas
import subprocess
from docx import Document
from abc import ABC, abstractmethod
from QuoteModel import QuoteMode
from ErrorTypes import InvalidLine, InvalidFileContent


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
    def validate_line(cls, line) -> None:
        """Validate a single line."""
        regex_pattern = r'"([^"]*)" - [a-zA-Z ]'
        if re.match(regex_pattern, line) is None:
            """Individual bad lines can fail w/o breaking code."""
            raise InvalidLine(f'Line "{line}" is invalid will not be added!')

        if len(line) > 100:
            raise InvalidLine(f'Line "{line}" is too long and will not be added!')

        pass

    @classmethod
    def validate_csv(cls, dataframe: pandas.DataFrame) -> None:
        """Ensure the CSV file has the correct headers."""
        expected_headers = ['body', 'author']
        if dataframe.columns.tolist() != expected_headers:
            raise InvalidFileContent((
                ".CSV file did not have expected headers!",
                "expected body,author",
                f'got {dataframe.columns.tolist()}'
                ))

        pass

    @classmethod
    def validate_cells(cls, *args) -> None:
        """Validate that length of a row <= 100."""
        big_string = ''
        for string in args:
            big_string = big_string + str(string)
        if len(big_string) > 100:
            raise InvalidFileContent(
                f'Length of quote "{big_string}" exceeds 100 chr. Will not be ingested.'
                )

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
        cls.validate_csv(dataframe)  # throws error based on headers
        wise_quotes = []  # to be returned
        for _, row in dataframe.iterrows():
            try:
                cls.validate_cells(row['body'], row['author'])
            except (InvalidFileContent) as e:
                print(e)
            else:
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
            try:
                cls.validate_line(line.text)  # Sometimes line are blank.
                string = line.text.replace('"', '')
                array = string.split("-")
                wise_quotes.append(
                    QuoteMode(array[0], array[1])
                )
            except InvalidLine as e:
                print(e)

        return wise_quotes


class PDFIngestor(IngestorInterface):
    """Ingest the pdf format."""

    extenstions = ['pdf']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Ingests a pdf. Converts to text."""
        cls.check_extention(path)
        outfile_path = '../_data/tmp/pdf-as-textf.txt'
        subprocess.call([
            'pdftotext', '-enc', 'UTF-8', '-simple',
            path, outfile_path
            ])
        # TXT file expected to be compatible with
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
            for line in word_array:
                try:
                    cls.validate_line(line)  # there are some blank lines
                    string = line.replace('"', '')
                    splitted_quote = string.split("-")
                    wise_quotes.append(
                        QuoteMode(splitted_quote[0], splitted_quote[1])
                        )
                except (InvalidLine, IndexError) as e: 
                    print(e)
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
