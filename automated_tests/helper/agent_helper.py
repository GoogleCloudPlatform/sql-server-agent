"""Google Cloud SQL Server Agent Helper.

This class will handle the installation and configuration of the SQL Server
Agent for our SQL Server Agent integration tests.

The service account must have IAM permissions for any calls necessary for
running the tests in the VM (see integration_test.py).
"""

import json
import time
import uuid

from absl import logging

from google3.cloud.cluster.testing.framework import virtual_machine as vm
from google3.pyglib import resources
from google3.third_party.sqlserveragent.automated_tests.base import exceptions as ex
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants
from google3.third_party.sqlserveragent.automated_tests.utils import bct_test_utils


class AgentHelper:
  """Facilitates installing, configuring and running SQL Server Agent."""

  def __init__(
      self,
      virtual_machine: vm.BigClusterVirtualMachine,
      windows: bool,
      repo_version: str,
  ):
    self._vm = virtual_machine
    self._windows = windows
    self._repo_version = repo_version
    self._gce_bucket = 'sql-server-agent-integration'
    self._g3_file_path = (
        f'gs://{self._gce_bucket}/{bct_test_utils.GetCurrentUser()}'
        '/google-cloud-sql-server-agent.exe'
    )

  def AddRepo(self):
    """Facilitates adding the repo to the created VM."""
    cmd = ''
    if self._windows:
      if self._repo_version == 'google3':
        cmd = (
            f'gsutil cp {self._g3_file_path}'
            f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_EXE_PATH}'
        )
        # Running gsutil is a special case that it steams output to err which
        # breaks the test.
        # Therefore we run the command without checking the error. Test will
        # break at InstallAgent() if gsutil cmd failed.
        logging.info('Running command: %s', cmd)
        _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
        logging.info('Output for running command: %s,\n err: %s', out, err)
        return
      repos = {
          'unstable': 'https://us-central1-googet.pkg.dev/projects/sql-server-solutions/repos/google-cloud-sql-server-agent-windows-x86-64-unstable',
          'stable': 'https://packages.cloud.google.com/yuck/repos/google-cloud-sql-server-agent-windows',
      }
      cmd = (
          'googet addrepo'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME} {repos[self._repo_version]}'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to add repo: {err}')

  def InstallAgent(self):
    """Facilitates installing the agent to the created VM."""

    cmd = ''
    if self._windows:
      if self._repo_version == 'google3':
        self.UpdateConfiguration('local.json')
        cmd = (
            f'& {sqlserveragent_test_constants.SQL_SERVER_AGENT_EXE_PATH}'
            ' --action=install'
        )
      else:
        cmd = (
            'googet -noconfirm install'
            f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME}'
        )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to install agent: {err}')
    self._VerifyAgentInstallation()

  def UpdateConfiguration(self, configuration: str):
    """Updates configuration file."""

    config_file = resources.GetResourceFilename(
        sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
        + f'configurations/{configuration}'
    )
    config_json = open(config_file, 'r').read()
    cmd = ''
    if self._windows:
      cmd = (
          'Set-Content -Path'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_CONFIG_PATH_WINDOWS} -Value'
          f' @"\n{config_json}\n"@ -Encoding ascii'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to update configuration: {err}')

  def ReadConfiguration(self) -> str:
    """Read configration file."""
    cmd = ''
    if self._windows:
      cmd = (
          'Get-Content -Path '
          f'{sqlserveragent_test_constants.SQL_SERVER_AGENT_CONFIG_PATH_WINDOWS}'
      )
    _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to read configuration: {err}')
    return out

  def StartService(self):
    """Start the service."""
    status = self._ServiceStatus()
    if status == 'Running':
      self._StopService()
      self._DeleteLogFile()
    cmd = ''
    if self._windows:
      cmd = (
          'Start-Service -Name'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME}'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.AgentExecutionError(f'Failed to start service: {err}')

  def VerifyAgentRunning(self):
    """Verify the agent is running without errors."""

    expected_status = ''
    if self._windows:
      expected_status = 'Running'

    status = self._ServiceStatus()
    if status != expected_status:
      raise ex.AgentExecutionError(f'Unexpected service status: {status}.')

    cmd = ''
    if self._windows:
      cmd = (
          'Get-Content -Path'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_LOG_PATH_WINDOWS}'
      )
    _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to read log: {err}')
    logging.info('Agent log: \n%s', out)
    has_err = False
    err_msg = ''
    for line in out.split('\n'):
      if line.startswith('{"level":"error"'):
        has_err = True
        err_msg = err_msg + line.strip() + '\n'

    if has_err:
      raise ex.AgentExecutionError(
          f'Unexpected error happens during the run. \n{err_msg}'
      )

  def VerifyEvaluationRun(self):
    """Veify the evaluation can run successfully after agent installation."""

    endpoint = 'workloadmanager.googleapis.com'
    if self._repo_version == 'google3':
      endpoint = 'autopush-workloadmanager.sandbox.googleapis.com'

    evaluation_id = self._ListEvaluations(endpoint)
    logging.info('Evaluation id: %s', evaluation_id)

    # generate execution uuid
    execution_id = uuid.uuid4()
    self._RunEvaluation(evaluation_id, execution_id, endpoint)
    execution_status = self._GetExecutionStatus(
        evaluation_id, execution_id, endpoint
    )

    if execution_status == 'FAILED':
      raise ex.EvaluationExecutionError(
          'Execution=%s of Evaluation=%s failed.'
          % (execution_id, evaluation_id)
      )

  def _ServiceStatus(self):
    cmd = ''
    if self._windows:
      cmd = (
          '$(Get-Service -Name'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME}).Status'
      )
    _, status, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to get service status: {err}')
    return status.split('\n')[0].strip()

  def _StopService(self):
    """Stop the service."""

    cmd = ''
    if self._windows:
      cmd = (
          'Stop-Service -Name'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME}'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.AgentExecutionError(f'Failed to stop service: {err}')

  def _DeleteLogFile(self):
    cmd = ''
    if self._windows:
      cmd = (
          'Remove-Item -Path'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_LOG_PATH_WINDOWS}'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to delete log: {err}')

  def _VerifyAgentInstallation(self):
    cmd = ''
    if self._windows:
      cmd = (
          'Get-Service -Name'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_NAME}'
      )
    _, _, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to verify agent installation: {err}')

  def _ListEvaluations(self, endpoint):
    """List the existing evaluations."""

    cmd = f"""
    $cred = gcloud auth application-default print-access-token
    $headers = @{{ "Authorization" = "Bearer $cred" }}

    try {{
      $Response =  Invoke-WebRequest -Method GET -Headers $headers -Uri "https://{endpoint}/v1/projects/wlm-for-sqlserver-integration/locations/us-central1/evaluations" -UseBasicParsing | Select-Object -Expand Content
      # This will only execute if the Invoke-WebRequest is successful.
      $StatusCode = $Response.StatusCode
      $Response
    }}
    catch {{
      echo '### Inside catch ###'
      $ErrorMessage = $_.Exception.Message
      echo '## ErrorMessage ##' $ErrorMessage
      $FailedItem = $_.Exception.ItemName
      echo '## FailedItem ##' $FailedItem
      $result = $_.Exception.Response.GetResponseStream()
      echo '## result2 ##' $result
      $reader = New-Object System.IO.StreamReader($result)
      echo '## reader ##' $reader
      $responseBody = $reader.ReadToEnd();
      echo '## responseBody ##' $responseBody
    }}
    """

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.EvaluationExecutionError(f'Failed to list evaluations: {err}')

    response_json = json.loads(out)
    evaluation_name = response_json['evaluations'][0]['name']
    logging.info('Rerun evaluation=%s', evaluation_name)

    evaluation_id = evaluation_name.split('/')[-1]
    logging.info('Evaluation id=%s', evaluation_id)
    return evaluation_id

  def _RunEvaluation(self, evaluation_id, execution_id, endpoint):
    """Run the evaluation.

    Args:
      evaluation_id: Evaluation ID.
      execution_id: Execution ID.
      endpoint: Endpoint of workload manager.

    Raises:
      EvaluationExecutionError
    """
    cmd = ''
    if self._windows:
      cmd = (
          'Invoke-WebRequest -Method POST -Headers $headers -Uri'
          f' "https://{endpoint}/v1/projects/wlm-for-sqlserver-integration/locations/us-central1/evaluations/{evaluation_id}/executions:run?execution_id={execution_id}"'
          ' -UseBasicParsing | Select-Object -Expand Content'
      )

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.EvaluationExecutionError(f'Failed to run evaluation: {err}')
    logging.info('Run evaluation result: %s', out)

  def _GetExecutionStatus(self, evaluation_id, execution_id, endpoint):
    """Get the execution status."""

    cmd = ''
    if self._windows:
      cmd = (
          'Invoke-WebRequest -Method GET -Headers $headers -Uri'
          f' "https://{endpoint}/v1/projects/wlm-for-sqlserver-integration/locations/us-central1/evaluations/{evaluation_id}/executions/{execution_id}"'
          ' -UseBasicParsing | Select-Object -Expand Content'
      )

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
    if err:
      raise ex.EvaluationExecutionError(
          f'Failed to get execution status: {err}'
      )

    # poll the evaluation execution result for 10 times
    for i in range(0, 10):
      execution_json = json.loads(out)
      execution_state = execution_json['state']
      logging.info('Execution status: %s. Count: %s', out, i + 1)
      if execution_state == 'RUNNING':
        # check execution status
        time.sleep(60)
        _, out, err = bct_test_utils.RunCommandOnVm(self._vm, cmd)
        if err:
          raise ex.EvaluationExecutionError(
              f'Failed to get execution status: {err}'
          )
      else:
        return execution_state
    return 'FAILED'
