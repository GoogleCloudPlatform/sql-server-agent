"""Common Methods for automated tests.

Defines common methods.
"""

import getpass

from google.cloud import secretmanager
from google.cloud import storage

from google3.cloud.cluster.testing.framework import users
from google3.cloud.cluster.testing.framework import virtual_machine
from google3.cloud.cluster.testing.framework import windows_test_utils
from google3.pyglib import resources


def GetCurrentUser() -> str:
  return getpass.getuser()


def UploadFileToBucket(gce_bucket, file_name, file_name_on_bucket):
  client = storage.Client(
      credentials=users.GetCredential(users.Users.GetCurrentEmail())
  )
  bucket = client.get_bucket(gce_bucket)
  blob = bucket.blob(file_name_on_bucket)
  blob.upload_from_filename(file_name, content_type='application/x-yaml')


def GetSecretValue(project_id, secret_id, version_id='latest'):
  """Gets the value of a secret from Secret Manager."""
  # Create the Secret Manager client.
  client = secretmanager.SecretManagerServiceClient(
      credentials=users.GetCredential(users.Users.GetCurrentEmail())
  )

  # Build the resource name of the secret version.
  name = f'projects/{project_id}/secrets/{secret_id}/versions/{version_id}'

  # Access the secret version.
  response = client.access_secret_version(name=name)

  # Return the decoded payload.
  return response.payload.data.decode('UTF-8')


def ReadFileContent(filename) -> str:
  with open(resources.GetResourceFilename(filename), 'r') as file_handle:
    file_content = file_handle.read()
  return file_content


def RunCommandOnVm(
    vm: virtual_machine.BigClusterVirtualMachine, cmd, linux: bool = False
):
  if not linux:
    return windows_test_utils.RunCommandOrRaiseOnFailure(vm, cmd)
  return vm.RunShellCommand(cmd)
