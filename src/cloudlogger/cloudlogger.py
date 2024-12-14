"""Creates a logging Class object.
Writes messages to S3
"""

import os
import boto3
import watchtower
import logging
import functools

logging.basicConfig(level=logging.INFO)

def log_wrapper(func):
    """ Configures a cloud watcher """
    client = boto3.client('logs', 'us-west-2')
    group_name = os.environ['LOG_GROUP']
    handler = watchtower.CloudWatchLogHandler(
        log_group_name=group_name,
        boto3_client=client,
        log_stream_name='all ec2'
        )

    @functools.wraps(func)
    def wrapper(*args, **kwargs):
        logger = logging.getLogger(func.__module__)
        logger.addHandler(handler)
        try:
            result = func(*args, **kwargs)
            logger.info(f'Exiting {func.__name__} with result: {result}')
        except Exception as e:
            logger.error(f'Exception in {func.__name__}: {e}')
            raise
    return wrapper
