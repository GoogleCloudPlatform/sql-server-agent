"""Module for SQL Server Agent automated test flags."""

from absl import flags

from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants

REPO_VERSION = flags.DEFINE_enum(
    'repo_version',
    default='none',
    enum_values=sqlserveragent_test_constants.VALID_REPO_VALUES,
    help='The version of the repo to use for the test',
)
