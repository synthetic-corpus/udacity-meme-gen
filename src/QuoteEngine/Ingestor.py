"""This File Contains a mod classes used to ingest quotes."""
import pandas
from abc import ABC, abstractmethod
from QuoteModel import QuoteMode


class IngestorInterface(ABC):
    """Is the Abstract class fo ingesters."""
    extenstions = []

    @classmethod
    def check_extention(cls, path: str) -> None:
        """Checks the file extension for a match."""
        extension = path.split('.')[-1]
        if(extension not in cls.extenstions):
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


class IngestCSV(IngestorInterface):
    """Ingest the CSV format."""
    extenstions = ['csv']

    @classmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Ingests quotes from file to a list."""
        cls.check_extention(path)
        dataframe = pandas.read_csv(path,header=0)
        wise_quotes = []  # to be returned
        for _,row in dataframe.iterrows():
            wise_quotes.append(
                QuoteMode(row['body'],row['author'])
            )
        return wise_quotes




class IngestDOCX(IngestorInterface):
    """Ingest the docx format."""
    extenstions = ['docx']

    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass


class IngestPDF(IngestorInterface):
    """Ingest the pdf format."""
    extenstions = ['pdf']

    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass


class IngestTXT(IngestorInterface):
    """Ingest the pdf format."""
    extenstions = ['txt']

    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass
