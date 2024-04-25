"""Google Cloud SQL Server Agent Load Test Helper.

This class will handle the hammerdb setups, sql scripts execution and the
running of virtual users.
"""

import datetime

from absl import logging
import numpy as np

from google3.cloud.cluster.testing.framework import virtual_machine as vm
from google3.cloud.cluster.testing.framework import windows_test_utils
from google3.third_party.sqlserveragent.automated_tests.base import exceptions as ex
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants
from google3.third_party.sqlserveragent.automated_tests.utils import bct_test_utils


class LoadtestHelper:
  """Loadtest helper class."""

  def __init__(
      self,
      vm_sqlserver: vm.BigClusterVirtualMachine,
      vm_hammerdb: vm.BigClusterVirtualMachine,
      hammerdb_version: str,
  ):
    current_time = datetime.datetime.now().strftime('%Y-%m-%d-%H-%M-%S')
    self._vm_sqlserver = vm_sqlserver
    self._vm_hammerdb = vm_hammerdb
    self._hammerdb_path = rf'C:\Program Files\HammerDB-{hammerdb_version}'
    self._project_number = '670032142250'
    self._secret_name = 'integration-test-pswd'
    self._background_job_name = 'run-load-test-background'
    self._test_result_path = r'C:\Windows\Temp\sqlPerf.log*'
    self._raw_data_file_name_prefix = f'loadtest-{current_time}'
    self._raw_data_bucket_path_prefix = (
        'gs://sql-server-agent-integration/loadtest-rawdata/'
    )
    self.mean_trans_per_sec = 0
    self.mean_lock_reqs_per_sec = 0
    self.mean_lock_timeouts_per_sec = 0
    self.mean_lock_waits_per_sec = 0
    self.mean_deadlocks_per_sec = 0
    self.mean_wait_time = 0
    self.mean_latch_wait_time = 0
    self.mean_trans_per_sec_agent_installed = 0
    self.mean_lock_reqs_per_sec_agent_installed = 0
    self.mean_lock_timeouts_per_sec_agent_installed = 0
    self.mean_lock_waits_per_sec_agent_installed = 0
    self.mean_deadlocks_per_sec_agent_installed = 0
    self.mean_wait_time_agent_installed = 0
    self.mean_latch_wait_time_agent_installed = 0

  def SetupHammerDB(self):
    file_name = 'setups.tcl'
    file_content = bct_test_utils.ReadFileContent(
        sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
        + f'load_test/{file_name}'
    )
    result = file_content.replace('%SERVER%', self._vm_sqlserver.GetPrivateIp())
    pswd = bct_test_utils.GetSecretValue(
        self._project_number, self._secret_name
    )
    result = result.replace('%PASSWORD%', pswd)
    self._RunTclScripts(file_name, result)

  def RunSqlScripts(self, file_name, background=False):
    """Runs sql scripts."""
    file_content = bct_test_utils.ReadFileContent(
        sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
        + f'load_test/{file_name}'
    )
    self._SaveFileToHammerDBFolder(file_name, file_content)

    pswd = bct_test_utils.GetSecretValue(
        self._project_number, self._secret_name
    )
    path = rf"'{self._hammerdb_path}\{file_name}'"
    cmd = (
        f'sqlcmd -U sa -P {pswd} -S {self._vm_sqlserver.GetPrivateIp()} -i'
        f' {path}'
    )
    if background:
      cmd = (
          'Start-Job -ScriptBlock {'
          + cmd
          + '}'
          + ' -Name '
          + self._background_job_name
      )
    logging.debug('Running command: %s', cmd)
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_hammerdb, cmd
    )
    if err:
      raise ex.RunSQLScriptError('Error found while running sql script: ', err)
    logging.debug('Output for running %s file:\n%s', file_name, out)

  def StopBackgroundJob(self):
    """Stops background job."""
    cmd = 'Stop-Job -Name ' + self._background_job_name
    logging.debug('Running command: %s', cmd)
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_hammerdb, cmd
    )
    if err:
      raise ex.RunSQLScriptError('Error found while stop background job: ', err)
    logging.debug(
        'Output for stopping job %s file:\n%s', self._background_job_name, out
    )

  def RunVirtualUsers(self):
    file_name = 'virtualusers.tcl'
    file_content = bct_test_utils.ReadFileContent(
        sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
        + f'load_test/{file_name}'
    )
    self._RunTclScripts(file_name, file_content)

  def _RunTclScripts(self, file_name, file_content):
    self._SaveFileToHammerDBFolder(file_name, file_content)
    cmd = (
        rf"cd '{self._hammerdb_path}' ; .\hammerdbcli tcl auto"
        rf' .\{file_name}'
    )
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_hammerdb, cmd
    )
    logging.info(f'Output for running {file_name}:\n%s', out)
    if err or 'error' in out:
      raise ex.HammerDbSetupError('Error found while hammerdb setups')

  def _SaveFileToHammerDBFolder(self, filename, file_content):
    """Saves file to HammerDB folder."""
    cmd = (
        'Set-Content -Path'
        rf" '{self._hammerdb_path}\{filename}' -Value"
        f' @"\n{file_content}\n"@ -Encoding ascii'
    )
    logging.debug('Running command: %s', cmd)
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_hammerdb, cmd
    )
    if err:
      raise ex.RunLoadTestError(
          f'Error found while saving file {filename}: ', err
      )
    logging.debug(f'Output for saving {filename} file:\n%s', out)

  def _GetLoadTestResult(self):
    """Gets load test result."""
    cmd = f'Get-Content -Path {self._test_result_path} -Encoding ascii'
    logging.debug('Running command: %s', cmd)
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_sqlserver, cmd
    )
    if err:
      raise ex.RunLoadTestError('Error found while getting test result: ', err)
    return out

  def UploadLoadTestResultToBucket(self, agent_installed=False):
    """Uploads load test result to bucket."""
    if agent_installed:
      cmd = (
          'gsutil cp'
          f' {self._test_result_path} {self._raw_data_bucket_path_prefix}{self._raw_data_file_name_prefix}-agent_installed.txt'
      )
    else:
      cmd = (
          'gsutil cp'
          f' {self._test_result_path} {self._raw_data_bucket_path_prefix}{self._raw_data_file_name_prefix}.txt'
      )
    logging.debug('Running command: %s', cmd)
    _, out, _ = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_sqlserver, cmd
    )
    logging.debug('Output for uploading test result:\n%s', out)

  def _DeleteTestResult(self):
    """Deletes load test result."""
    cmd = f'Remove-Item -Path {self._test_result_path}'
    logging.debug('Running command: %s', cmd)
    _, out, err = windows_test_utils.RunCommandOrRaiseOnFailure(
        self._vm_sqlserver, cmd
    )
    if err:
      raise ex.RunLoadTestError('Error found while deleting test result: ', err)
    logging.debug('Output for deleting test result:\n%s', out)

  def ProcessLoadTestResult(self, agent_installed=False):
    """Processes load test result.

    Calculate the average and standard deviation for each metric w/o agent
    installed.

    Args:
      agent_installed: If the test is running with sql server agent installed.
    """
    trans_per_sec = []
    lock_reqs_per_sec = []
    lock_timeouts_per_sec = []
    lock_waits_per_sec = []
    deadlocks_per_sec = []
    avg_wait_time = []
    avg_latch_wait_time = []

    result = self._GetLoadTestResult().split('\n')[1:]
    for row in result:
      cols = row.split(',')
      if len(cols) != 9:
        continue
      trans_per_sec.append(int(cols[0]))
      lock_reqs_per_sec.append(int(cols[3]))
      lock_timeouts_per_sec.append(int(cols[4]))
      lock_waits_per_sec.append(int(cols[5]))
      deadlocks_per_sec.append(int(cols[6]))
      avg_wait_time.append(int(cols[7]))
      avg_latch_wait_time.append(int(cols[8]))

    mean_trans_per_sec = round(np.mean(trans_per_sec))
    mean_lock_reqs_per_sec = round(np.mean(lock_reqs_per_sec))
    mean_lock_timeouts_per_sec = round(np.mean(lock_timeouts_per_sec))
    mean_lock_waits_per_sec = round(np.mean(lock_waits_per_sec))
    mean_deadlocks_per_sec = round(np.mean(deadlocks_per_sec))
    mean_wait_time = round(np.mean(avg_wait_time))
    mean_latch_wait_time = round(np.mean(avg_latch_wait_time))

    if agent_installed:
      self.mean_trans_per_sec_agent_installed = mean_trans_per_sec
      self.mean_lock_reqs_per_sec_agent_installed = mean_lock_reqs_per_sec
      self.mean_lock_timeouts_per_sec_agent_installed = (
          mean_lock_timeouts_per_sec
      )
      self.mean_lock_waits_per_sec_agent_installed = mean_lock_waits_per_sec
      self.mean_deadlocks_per_sec_agent_installed = mean_deadlocks_per_sec
      self.mean_wait_time_agent_installed = mean_wait_time
      self.mean_latch_wait_time_agent_installed = mean_latch_wait_time
    else:
      self.mean_trans_per_sec = mean_trans_per_sec
      self.mean_lock_reqs_per_sec = mean_lock_reqs_per_sec
      self.mean_lock_timeouts_per_sec = mean_lock_timeouts_per_sec
      self.mean_lock_waits_per_sec = mean_lock_waits_per_sec
      self.mean_deadlocks_per_sec = mean_deadlocks_per_sec
      self.mean_wait_time = mean_wait_time
      self.mean_latch_wait_time = mean_latch_wait_time
    self._DeleteTestResult()

  def GenerateLoadTestResult(self):
    """Generates load test result."""
    diff_trans_per_sec = abs(
        self.mean_trans_per_sec - self.mean_trans_per_sec_agent_installed
    )
    diff_lock_reqs_per_sec = abs(
        self.mean_lock_reqs_per_sec
        - self.mean_lock_reqs_per_sec_agent_installed
    )
    diff_lock_timeouts_per_sec = abs(
        self.mean_lock_timeouts_per_sec
        - self.mean_lock_timeouts_per_sec_agent_installed
    )
    diff_lock_waits_per_sec = abs(
        self.mean_lock_waits_per_sec
        - self.mean_lock_waits_per_sec_agent_installed
    )
    diff_deadlocks_per_sec = abs(
        self.mean_deadlocks_per_sec
        - self.mean_deadlocks_per_sec_agent_installed
    )
    diff_wait_time = abs(
        self.mean_wait_time - self.mean_wait_time_agent_installed
    )
    diff_latch_wait_time = abs(
        self.mean_latch_wait_time - self.mean_latch_wait_time_agent_installed
    )

    result = ''
    result += f'mean_trans_per_sec: {self.mean_trans_per_sec}\n'
    result += (
        'mean_trans_per_sec_agent_installed:'
        f' {self.mean_trans_per_sec_agent_installed}\n'
    )
    if self.mean_trans_per_sec != 0:
      result += (
          'diff_trans_per_sec:'
          f' {diff_trans_per_sec * 100 / self.mean_trans_per_sec}\n'
      )
    result += f'mean_lock_reqs_per_sec: {self.mean_lock_reqs_per_sec}\n'
    result += (
        'mean_lock_reqs_per_sec_agent_installed:'
        f' {self.mean_lock_waits_per_sec_agent_installed}\n'
    )
    if self.mean_lock_reqs_per_sec != 0:
      result += (
          'diff_lock_reqs_per_sec:'
          f' {diff_lock_reqs_per_sec * 100 / self.mean_lock_reqs_per_sec}\n'
      )
    result += f'mean_lock_timeouts_per_sec: {self.mean_lock_timeouts_per_sec}\n'
    result += (
        'mean_lock_timeouts_per_sec_agent_installed:'
        f' {self.mean_lock_timeouts_per_sec_agent_installed}\n'
    )
    if self.mean_lock_timeouts_per_sec != 0:
      result += (
          'diff_lock_timeouts_per_sec:'
          f' {diff_lock_timeouts_per_sec * 100 / self.mean_lock_timeouts_per_sec}\n'
      )
    result += f'mean_lock_waits_per_sec: {self.mean_lock_waits_per_sec}\n'
    result += (
        'mean_lock_waits_per_sec_agent_installed:'
        f' {self.mean_lock_waits_per_sec_agent_installed}\n'
    )
    if self.mean_lock_waits_per_sec != 0:
      result += (
          'diff_lock_waits_per_sec:'
          f' {diff_lock_waits_per_sec * 100 / self.mean_lock_waits_per_sec}\n'
      )
    result += f'mean_deadlocks_per_sec: {self.mean_deadlocks_per_sec}\n'
    result += (
        'mean_deadlocks_per_sec_agent_installed:'
        f' {self.mean_deadlocks_per_sec_agent_installed}\n'
    )
    if self.mean_deadlocks_per_sec != 0:
      result += (
          'diff_deadlocks_per_sec:'
          f' {diff_deadlocks_per_sec * 100 / self.mean_deadlocks_per_sec}\n'
      )
    result += f'mean_wait_time: {self.mean_wait_time}\n'
    result += (
        'mean_wait_time_agent_installed:'
        f' {self.mean_wait_time_agent_installed}\n'
    )
    if self.mean_wait_time != 0:
      result += (
          'diff_wait_time:'
          f' {diff_wait_time * 100 / self.mean_wait_time}\n'
      )
    result += f'mean_latch_wait_time: {self.mean_latch_wait_time}\n'
    result += (
        'mean_latch_wait_time_agent_installed:'
        f' {self.mean_latch_wait_time_agent_installed}\n'
    )
    if self.mean_latch_wait_time != 0:
      result += (
          'diff_latch_wait_time:'
          f' {diff_latch_wait_time * 100 / self.mean_latch_wait_time}\n'
      )
    logging.info('Load test result: \n%s', result)
    logging.info(
        'Please find the raw data %s.\n',
        'https://pantheon.corp.google.com/storage/browser/sql-server-agent-integration/loadtest-rawdata',
    )
