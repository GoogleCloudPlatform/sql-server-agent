"""Integration tests creation for SQL Server Agent using BCT (Big Cluster Test) framework.

This class will handle the creation of BCT test cases and associated VMs.

The service account must have IAM permissions for any calls necessary for
running the tests in the VM.
"""

import time

import google3

from absl import flags

from google3.cloud.cluster.testing.framework import bigclustertest
from google3.cloud.cluster.testing.framework import virtual_machine
from google3.cloud.cluster.testing.framework import windows_test_utils
from google3.third_party.sqlserveragent.automated_tests.helper import integrationtest_helper
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_constants
from google3.third_party.sqlserveragent.automated_tests.parameters import sqlserveragent_test_flags

FLAGS = flags.FLAGS


class IntegrationTest(bigclustertest.TestCase):

  @classmethod
  def setUpClass(cls):
    super().setUpClass()
    vm_win_local = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='integration-win-agent-local',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_WIN,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_win_remote = windows_test_utils.CreateWindowsVm(
        cls=cls,
        name='integration-win-agent-remote',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_WIN,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_rhel = cls.CreateVm(
        name='integration-rhel-agent',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_RHEL,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )
    vm_suse = cls.CreateVm(
        name='integration-suse-agent',
        raw_names=True,
        wait_for_instance_ready=True,
        vm_image_path=sqlserveragent_test_constants.VM_IMAGE_PREFIX
        + sqlserveragent_test_constants.VM_IMAGE_SUSE,
        network_configs=[
            virtual_machine.NetworkConfig(
                network=sqlserveragent_test_constants.NETWORK,
                subnetwork=sqlserveragent_test_constants.SUBNETWORK,
            )
        ],
    )

    # Set up service account scopes to call workload manager evaluation API
    # See from http://google3/google/cloud/workloadmanager/workloadmanager.yaml
    vm_win_local.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_win_remote.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_rhel.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    vm_suse.AddRobot(
        FLAGS.service_account_name,
        sqlserveragent_test_constants.SERVICE_ACCOUNT_SCOPES,
    )
    # clear the ssh key added to the VM by BCT so the existing key on windows
    # works fine.
    vm_rhel.ClearSshKeys()
    vm_suse.ClearSshKeys()

    # insert VMs
    vm_win_local.Insert()
    vm_win_remote.Insert()
    vm_rhel.Insert()
    vm_suse.Insert()

    cls.integrationtest_helper = integrationtest_helper.IntegrationHelper(
        sqlserveragent_test_flags.REPO_VERSION.value,
        vm_win_local,
        vm_win_remote,
        vm_rhel,
        vm_suse,
    )
    time.sleep(300)

  def testLocalCollectionRhel(self):
    """Test if sql server agent run local collection properly on RHEL."""
    self._runAgentAndVerify(sqlserveragent_test_constants.VM_ID_RHEL)

  def testLocalCollectionSuse(self):
    """Test if sql server agent run local collection properly on SUSE."""
    self._runAgentAndVerify(sqlserveragent_test_constants.VM_ID_SUSE)

  def testLocalCollectionWin(self):
    """Test if sql server agent run local collection properly on windows."""
    self._runAgentAndVerify(sqlserveragent_test_constants.VM_ID_WIN_LOCAL)

  def testRemoteCollection(self):
    """Test if sql server agent run remote collection properly on windows."""
    # remote collection
    self._runAgentAndVerify(
        sqlserveragent_test_constants.VM_ID_WIN_REMOTE,
        'remote-integration.json',
    )

  # def testWorkloadManagerEvaluation(self):
  #   self.integrationtest_helper.VerifyEvaluationRun()

  def _runAgentAndVerify(self, vm_id, configuration: str = 'local.json'):
    self.integrationtest_helper.SetUpAgent(vm_id, configuration)
    self.integrationtest_helper.VerifyAgentRunning(vm_id)


if __name__ == '__main__':
  bigclustertest.main()
