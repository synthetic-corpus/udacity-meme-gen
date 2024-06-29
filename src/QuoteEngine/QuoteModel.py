"""This Class exists to standardize a quote object.
   This will be used by the ingestor methods and
   functions.
"""


class QuoteModel:
    """Class is quote and an author."""

    def __init__(self, quote: str, author: str):
        self._quote = quote.strip()
        self._author = author.strip()

    def __str__(self):
        wise_saying = f'"{self._quote}" - {self._author}'
        return wise_saying

    def __repr__(self):
        wise_saying = f'"{self._quote}" - {self._author}'
        return wise_saying

    def replace_quote(self, quote: str):
        """Can replace the quote on this instance."""
        self._quote = quote.strip()

    def replace_author(self, author: str):
        """May swap out author if you like."""
        self._author = author.strip()

    @property
    def length(self):
        """Retrieves the length of the quote."""
        return len(f'"{self._quote}" - {self._author}')
