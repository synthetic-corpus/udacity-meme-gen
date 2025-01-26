""" This Class is expermiental
Will Eventually Write events to DynamoDB.
For now, it just sends messages to CloudFormations
"""
from datetime import datetime
import Boto3
from cloudlogger import log_wrapper, cloud_logger


class DatabaseAccess(Boto3.client):
    """ Creates an object that
    Writes to a table"""
    def __init__(self, table, aws_region):
        """ @table the name of the DynamoDBTable to write to"""
        super().__init__(
            service_name='dynamodb',
            region_name=aws_region
        )
        self.table = table

    @staticmethod
    def record_processing(id: str,
                          source: str,
                          outputs: list[str],
                          font: str,
                          text: str = 'no text',
                          author: str= 'no author') -> None:
        """ Logs to cloud logger
            Exists here for testing only.
        """
        message = f'UUID: {id} file {source} -> {outputs} \
            with {text} - {author} ({font})'
        cloud_logger.info(message)

    @log_wrapper
    def dynamo_putlog(self, id:str, source: str,outputs: list[str],
                          font: str,text: str = 'no text',
                          author: str= 'no author') -> None:
        """ Writes to the Dynamo DB Table"""
        item = {
            "ID": {
                "S": id
            },
            "SourceFile": {
                "S": source
            },
            "Font": {
                "S": font
            },
            "Text": {
                "S": text
            },
            "Author": {
                "S": author
            },
            "Outputs": {
                "SS": outputs
            }
        }
        try:
            self.put_item(TableName=self.table,Item=item)
        except Exception as e:
            cloud_logger.error(f'{type(e).__name__} - {e}')