"""This File Contains a mod classes used to ingest quotes."""
from abc import ABC, abstractmethod
from QuoteModel import QuoteMode


class IngestorInterface(ABC):
    """Is the Abstract class fo ingesters."""
    extenstions = []

    @classmethod
    def check_extention(cls, path: str) -> bool:
        """Checks the file extension for a match."""
        extension = path.split('.')[-1]
        return extension in cls.extenions

    @classmethod
    @abstractmethod
    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass


class IngestCSV(IngestorInterface):
    """Ingest the CSV format."""
    extenstions = ['csv']

    def ingest(cls, path: str) -> list[QuoteMode]:
        """Each Realization will override."""
        pass


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
