""" This Class is expermiental
Will Eventually Write events to DynamoDB.
For now, it just sends messages to CloudFormations
"""

from cloudlogger import log_wrapper, cloud_logger


class DatabaseAccess():

    @staticmethod
    def record_processing(id: str,
                          source: str,
                          ouputs: list[str],
                          font: str,
                          text: str = 'no text',
                          author: str= 'no author') -> None:
        """ Logs the Text source file etc from Meme """
        message = f'UUID: {id} file {source} -> {ouputs} \
            with {text} - {author} ({font})'
        cloud_logger.info(message)
