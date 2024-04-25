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


class IntegrationHelper:
  """Facilitates installing, configuring and running SQL Server Agent."""

  def __init__(
      self,
      repo_version: str,
      vm_win_local: vm.BigClusterVirtualMachine,
      vm_win_remote: vm.BigClusterVirtualMachine,
      vm_rhel: vm.BigClusterVirtualMachine,
      vm_suse: vm.BigClusterVirtualMachine,
  ):
    self._repo_version = repo_version
    self._vm_win_local = vm_win_local
    self._vm_win_remote = vm_win_remote
    self._vm_rhel = vm_rhel
    self._vm_suse = vm_suse
    self._gce_bucket = 'sql-server-agent-integration'
    self._g3_file_path_win = (
        f'gs://{self._gce_bucket}/{bct_test_utils.GetCurrentUser()}'
        '/google-cloud-sql-server-agent.exe'
    )
    self._g3_file_path_linux = (
        f'gs://{self._gce_bucket}/{bct_test_utils.GetCurrentUser()}'
        '/google-cloud-sql-server-agent'
    )

  def SetUpAgent(self, target_vm_id, configuration_name: str = 'local.json'):
    """Sets up the agent on the vm.

    Args:
      target_vm_id: specify which vm to set up.
      configuration_name: name of configuration file. default configuration file
        name is local.json.

    Raises:
      RunCommandError
    """
    config = open(
        resources.GetResourceFilename(
            sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
            + f'configurations/{configuration_name}'
        ),
        'r',
    ).read()

    if (
        target_vm_id == sqlserveragent_test_constants.VM_ID_WIN_LOCAL
        or target_vm_id == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
    ):
      script_path = 'C:\\temp\\setup_agent_win.ps1'
      script = open(
          resources.GetResourceFilename(
              sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
              + 'scripts/agent-setup-win.ps1'
          ),
          'r',
      ).read()
      script = script.replace('$', '`$')
      script = script.replace('@', '`@')
      cmd = (
          f'New-Item -Path {script_path} -Type file -Force;Set-Content -Path'
          f' {script_path} -Value @"\n{script}\n"@ -Encoding'
          f' ascii;{script_path} @"\n{config}\n"@'
          f' {self._repo_version} {self._g3_file_path_win}'
      )
    else:
      if target_vm_id == sqlserveragent_test_constants.VM_ID_RHEL:
        script_name = 'agent-setup-rhel.sh'
        vm_ip = self._vm_rhel.GetPrivateIp()
      else:
        script_name = 'agent-setup-suse.sh'
        vm_ip = self._vm_suse.GetPrivateIp()
      script = open(
          resources.GetResourceFilename(
              sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
              + f'scripts/{script_name}'
          ),
          'r',
      ).read()
      script = script.replace('"', '\\"')
      script = script.replace('$', '`$')
      config = config.replace('"', '\\"')
      cmd = f"""ssh -o StrictHostKeyChecking=no -i C:\\Users\\local-user\\.ssh\\id_rsa local-user@{vm_ip} @"
echo '{script}' > {script_name}
chmod 775 {script_name}
sudo ./{script_name} {self._repo_version} '{config}' {self._g3_file_path_linux}
"@
"""

    # only for remote collection the command runs on vm_win_remote
    # all other vms the command will be running on vm_win_local
    vm_agent = (
        self._vm_win_remote
        if target_vm_id == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
        else self._vm_win_local
    )
    _, _, _ = bct_test_utils.RunCommandOnVm(vm_agent, cmd)

  def VerifyAgentRunning(self, target_vm_id):
    """Verify the agent is running without errors."""

    if (
        target_vm_id == sqlserveragent_test_constants.VM_ID_WIN_LOCAL
        or target_vm_id
        == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
    ):
      cmd = (
          'Get-Content -Path'
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_LOG_PATH_WINDOWS}'
      )
    else:
      if target_vm_id == sqlserveragent_test_constants.VM_ID_RHEL:
        vm_ip = self._vm_rhel.GetPrivateIp()
      else:
        vm_ip = self._vm_suse.GetPrivateIp()
      cmd = (
          'ssh -o StrictHostKeyChecking=no -i'
          ' C:\\Users\\local-user\\.ssh\\id_rsa'
          f' local-user@{vm_ip} @"\n'
          'sudo cat '
          f' {sqlserveragent_test_constants.SQL_SERVER_AGENT_LOG_PATH_LINUX}\n'
          '"@'
      )
    vm_agent = (
        self._vm_win_remote
        if target_vm_id == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
        else self._vm_win_local
    )
    _, out, err = bct_test_utils.RunCommandOnVm(vm_agent, cmd)
    if err:
      raise ex.RunCommandError(f'Failed to read log: {err}')
    logging.info('Agent log from vm_id %d: \n%s', target_vm_id, out)
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

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm_win_local, cmd)
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

    cmd = (
        'Invoke-WebRequest -Method POST -Headers $headers -Uri'
        f' "https://{endpoint}/v1/projects/wlm-for-sqlserver-integration/locations/us-central1/evaluations/{evaluation_id}/executions:run?execution_id={execution_id}"'
        ' -UseBasicParsing | Select-Object -Expand Content'
    )

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm_win_local, cmd)
    if err:
      raise ex.EvaluationExecutionError(f'Failed to run evaluation: {err}')
    logging.info('Run evaluation result: %s', out)

  def _GetExecutionStatus(self, evaluation_id, execution_id, endpoint):
    """Get the execution status."""

    cmd = (
        'Invoke-WebRequest -Method GET -Headers $headers -Uri'
        f' "https://{endpoint}/v1/projects/wlm-for-sqlserver-integration/locations/us-central1/evaluations/{evaluation_id}/executions/{execution_id}"'
        ' -UseBasicParsing | Select-Object -Expand Content'
    )

    _, out, err = bct_test_utils.RunCommandOnVm(self._vm_win_local, cmd)
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
        _, out, err = bct_test_utils.RunCommandOnVm(self._vm_win_local, cmd)
        if err:
          raise ex.EvaluationExecutionError(
              f'Failed to get execution status: {err}'
          )
      else:
        return execution_state
    return 'FAILED'
