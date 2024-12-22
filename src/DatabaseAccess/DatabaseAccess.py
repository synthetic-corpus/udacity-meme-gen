""" This Class is expermiental
Will Eventually Write events to DynamoDB.
For now, it just sends messages to CloudFormations
"""

from cloudlogger import log_wrapper, cloud_logger


class DatabaseAccess():

    @staticmethod
    @log_wrapper
    def record_processing(id: str,
                          source: str,
                          ouputs: list[str],
                          text: str = "no text") -> None:
        """ Logs the Text source file etc from Meme """
        message = f'UUID: {id} file {source} -> {ouputs} with {text}'
        cloud_logger.info(message)
