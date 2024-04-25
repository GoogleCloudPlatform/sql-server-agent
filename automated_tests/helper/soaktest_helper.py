"""Google Cloud SQL Server Agent Soak Test Helper.

This class contains helper functions that are used during the agent Soak test.
"""

from google3.cloud.cluster.testing.framework import virtual_machine as vm
from google3.pyglib import resources
from google3.third_party.sqlserveragent.automated_tests.base import exceptions as ex
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants
from google3.third_party.sqlserveragent.automated_tests.utils import bct_test_utils

PS1_FILENAME = 'upload_log_file.ps1'
SH_FILENAME = 'upload_log_file.sh'


class SoakTestHelper:
  """Helper class for the agent Soak test."""

  def __init__(
      self,
      vm_win_local: vm.BigClusterVirtualMachine,
      vm_win_remote: vm.BigClusterVirtualMachine,
      vm_rhel: vm.BigClusterVirtualMachine,
      vm_suse: vm.BigClusterVirtualMachine,
  ):
    self.vm_win_local = vm_win_local
    self.vm_win_remote = vm_win_remote
    self.vm_rhel = vm_rhel
    self.vm_suse = vm_suse

  def RunUploadLogFileScript(self, vm_id):
    """Runs the upload_log_file script on the created VMs."""
    if (
        vm_id == sqlserveragent_test_constants.VM_ID_WIN_LOCAL
        or vm_id == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
    ):
      script_content = open(
          resources.GetResourceFilename(
              sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
              + f'scripts/{PS1_FILENAME}'
          ),
          'r',
      ).read()
      # replace special character $ with `$
      script_content = script_content.replace('$', '`$')
      ps1_path = f'C:\\temp\\{PS1_FILENAME}'
      cmd = (
          f'New-Item -Path {ps1_path} -Type file -Force;'
          f'Set-Content -Path {ps1_path} -Value'
          f' @"\n{script_content}\n"@ -Encoding ascii -Force;'
          f'Start-Job -FilePath {ps1_path}'
      )
    else:
      if vm_id == sqlserveragent_test_constants.VM_ID_RHEL:
        vm_ip = self.vm_rhel.GetPrivateIp()
      else:
        vm_ip = self.vm_suse.GetPrivateIp()
      script_content = open(
          resources.GetResourceFilename(
              sqlserveragent_test_constants.AGENT_FOR_SQL_SERVER_AGENT_DIR
              + f'scripts/{SH_FILENAME}'
          ),
          'r',
      ).read()
      script_content = script_content.replace('$', '`$')
      script_content = script_content.replace('"', '\\"')
      cmd = f"""ssh -o StrictHostKeyChecking=no -i C:\\Users\\local-user\\.ssh\\id_rsa local-user@{vm_ip} @"
echo '{script_content}' > {SH_FILENAME}
chmod 775 {SH_FILENAME}
nohup ./{SH_FILENAME} > /dev/null 2>&1 &
"@
"""

    vm_agent = (
        self.vm_win_remote
        if vm_id == sqlserveragent_test_constants.VM_ID_WIN_REMOTE
        else self.vm_win_local
    )
    _, _, err = bct_test_utils.RunCommandOnVm(vm_agent, cmd)
    if err:
      raise ex.RunCommandError(
          f'error when running the command: {cmd}\n on vm id {vm_id}. ', err
      )
