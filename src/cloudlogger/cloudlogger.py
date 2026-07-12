"""CloudWatch logging helpers for the meme generator app."""

import functools
import logging
import os
import sys

import boto3
import watchtower

logging.basicConfig(level=logging.INFO)

_region = os.environ.get('SOURCE_REGION') or os.environ.get('AWS_REGION') or 'us-west-2'
_client = boto3.client('logs', _region)
_group_name = os.environ['LOG_GROUP']
_handler = watchtower.CloudWatchLogHandler(
    log_group_name=_group_name,
    boto3_client=_client,
    log_stream_name='meme-generator',
    create_log_group=False,
)
cloud_logger = logging.getLogger('cloud logs')
cloud_logger.setLevel(logging.INFO)
cloud_logger.addHandler(_handler)
# Avoid duplicate lines if root handlers are also configured.
cloud_logger.propagate = False


class _TeeToCloudWatch:
    """Mirror writes to the original stream and to CloudWatch."""

    def __init__(self, logger, stream):
        self.logger = logger
        self.stream = stream
        self._buf = ''

    def write(self, data):
        if not isinstance(data, str):
            data = str(data)
        self.stream.write(data)
        self._buf += data
        while '\n' in self._buf:
            line, self._buf = self._buf.split('\n', 1)
            if line.strip():
                self.logger.info(line)

    def flush(self):
        if self._buf.strip():
            self.logger.info(self._buf.rstrip())
            self._buf = ''
        self.stream.flush()
        for handler in self.logger.handlers:
            handler.flush()

    def isatty(self):
        return False


def capture_stdout():
    """Send process stdout to CloudWatch while still printing locally."""
    if not isinstance(sys.stdout, _TeeToCloudWatch):
        sys.stdout = _TeeToCloudWatch(cloud_logger, sys.__stdout__)
    if not isinstance(sys.stderr, _TeeToCloudWatch):
        sys.stderr = _TeeToCloudWatch(cloud_logger, sys.__stderr__)


def log_wrapper(func):
    """Log enter/exit/exceptions for a function without changing its contract.

    Safe under Flask: use ``@app.route(...)`` above ``@log_wrapper`` so the
    route registers the wrapped view.
    """

    @functools.wraps(func)
    def wrapper(*args, **kwargs):
        cloud_logger.info(f'Entering {func.__name__}')
        try:
            result = func(*args, **kwargs)
            cloud_logger.info(f'Exiting {func.__name__} successfully')
            return result
        except Exception as e:
            cloud_logger.error(
                f'Exception in {func.__name__}: {type(e).__name__}: {e}',
                exc_info=True,
            )
            raise
        finally:
            for handler in cloud_logger.handlers:
                handler.flush()

    return wrapper
