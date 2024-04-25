"""Custom exceptions for SQL Server Agent BCT Tests.

File defines the custom exceptions that can be used across all tests.
"""


class UnsupportedOsForError(Exception):
  """Custom exception for when an unrecognized OS image is found."""


class UnsupportedRepoError(Exception):
  """Custom exception for when an unrecognized repo version is found."""


class SshCommandError(Exception):
  """Custom exception for when SSH command executions fails."""


class PackageInstallError(Exception):
  """Custom exception for when package installation on the remote VM fails."""


class RunCommandError(Exception):
  """Custom exception for when running the command fails."""


class AgentExecutionError(Exception):
  """Custom exception for when running the agent encounters any errors."""


class EvaluationExecutionError(Exception):
  """Custom exception for when running the evaluation encounters any errors."""


class InvalidArgumentError(Exception):
  """Custom exception for when an invalid argument is used."""


class HammerDbSetupError(Exception):
  """Custom exception for when HammerDB setup fails."""


class RunSQLScriptError(Exception):
  """Custom exception for when running SQL script fails."""


class RunLoadTestError(Exception):
  """Custom exception for when running load test fails."""
